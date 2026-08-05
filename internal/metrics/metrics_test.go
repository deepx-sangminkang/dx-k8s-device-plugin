package metrics

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

func scrape(t *testing.T, list ListFunc) string {
	t.Helper()
	rr := httptest.NewRecorder()
	NewHandler(list).ServeHTTP(rr, httptest.NewRequest("GET", "/metrics", nil))
	return rr.Body.String()
}

func TestHandler_ExposesDeviceHealth(t *testing.T) {
	devs := []dxdevice.Device{
		{ID: 0, Name: "dxrt0", Product: "M1", PCIe: "Gen3 X4 [85:00:00]", Healthy: true,
			Cores: []dxdevice.Core{{ID: 0, TemperatureC: 43, VoltageMV: 750, ClockMHz: 1000}}},
		{ID: 1, Name: "dxrt1", Product: "M1", PCIe: "Gen3 X4 [86:00:00]", Healthy: false},
	}
	body := scrape(t, func() ([]dxdevice.Device, error) { return devs, nil })

	for _, want := range []string{
		"deepx_npu_up 1",
		`deepx_npu_device_healthy{device="dxrt0",model="M1",pcie="Gen3 X4 [85:00:00]"} 1`,
		`deepx_npu_device_healthy{device="dxrt1",model="M1",pcie="Gen3 X4 [86:00:00]"} 0`,
		`deepx_npu_core_temperature_celsius{core="0",device="dxrt0"} 43`,
		`deepx_npu_core_voltage_millivolts{core="0",device="dxrt0"} 750`,
		`deepx_npu_core_clock_mhz{core="0",device="dxrt0"} 1000`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q\n---\n%s", want, body)
		}
	}
}

func TestHandler_ListFailure(t *testing.T) {
	body := scrape(t, func() ([]dxdevice.Device, error) { return nil, errors.New("boom") })
	if !strings.Contains(body, "deepx_npu_up 0") {
		t.Errorf("list failure must report deepx_npu_up 0\n---\n%s", body)
	}
}
