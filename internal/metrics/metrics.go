// Package metrics serves the deepx_npu_* Prometheus metrics that the dx-npu
// Helm chart scrapes (headless Service targeting the named "metrics" port).
// Enabled via METRICS_ADDR; the chart sets it to "" when metrics.enabled=false.
package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

// ListFunc supplies the current device list; dxdevice.List in production.
type ListFunc func() ([]dxdevice.Device, error)

type collector struct {
	list     ListFunc
	up       *prometheus.Desc
	healthy  *prometheus.Desc
	coreTemp *prometheus.Desc
	coreVolt *prometheus.Desc
	coreClk  *prometheus.Desc
}

// NewHandler returns an http.Handler rendering the metrics from list. Each
// scrape re-enumerates, so values are always current (no staleness window).
func NewHandler(list ListFunc) http.Handler {
	coreLabels := []string{"device", "core"}
	reg := prometheus.NewRegistry()
	reg.MustRegister(&collector{
		list: list,
		up: prometheus.NewDesc("deepx_npu_up",
			"1 when NPU enumeration succeeds on this node.", nil, nil),
		healthy: prometheus.NewDesc("deepx_npu_device_healthy",
			"1 when the card is healthy (present in sysfs and dxrt-cli).",
			[]string{"device", "model", "pcie"}, nil),
		coreTemp: prometheus.NewDesc("deepx_npu_core_temperature_celsius",
			"Per-core temperature.", coreLabels, nil),
		coreVolt: prometheus.NewDesc("deepx_npu_core_voltage_millivolts",
			"Per-core voltage.", coreLabels, nil),
		coreClk: prometheus.NewDesc("deepx_npu_core_clock_mhz",
			"Per-core clock.", coreLabels, nil),
	})
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.up, c.healthy, c.coreTemp, c.coreVolt, c.coreClk} {
		ch <- d
	}
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	devs, err := c.list()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1)
	for _, d := range devs {
		v := 0.0
		if d.Healthy {
			v = 1
		}
		ch <- prometheus.MustNewConstMetric(c.healthy, prometheus.GaugeValue, v,
			d.Name, d.Product, d.PCIe)
		for _, core := range d.Cores {
			id := strconv.Itoa(core.ID)
			ch <- prometheus.MustNewConstMetric(c.coreTemp, prometheus.GaugeValue,
				float64(core.TemperatureC), d.Name, id)
			ch <- prometheus.MustNewConstMetric(c.coreVolt, prometheus.GaugeValue,
				float64(core.VoltageMV), d.Name, id)
			ch <- prometheus.MustNewConstMetric(c.coreClk, prometheus.GaugeValue,
				float64(core.ClockMHz), d.Name, id)
		}
	}
}

// Serve blocks on ListenAndServe when addr is non-empty; no-op otherwise.
func Serve(addr string, list ListFunc) error {
	if addr == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", NewHandler(list))
	return http.ListenAndServe(addr, mux)
}
