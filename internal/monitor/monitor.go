// Package monitor keeps the on-disk CDI spec in sync with the NPUs actually
// present on the node: it re-enumerates on an interval and rewrites
// /etc/cdi/deepx.json so containerd always has a current spec to resolve
// Allocate's deepx.ai/dx-m1=<id> references against.
package monitor

import (
	"context"
	"log"
	"time"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/cdi"
	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

// Seams overridden in tests.
var (
	listFunc  = dxdevice.List
	writeFunc = cdi.Write
)

// Monitor regenerates the CDI spec for the node's NPUs.
type Monitor struct {
	CDIDir   string        // where deepx.json is written, e.g. /etc/cdi
	Libs     []string      // host runtime libs to inject (cdi.DefaultLibs, or nil for thin)
	Interval time.Duration // re-scan period
}

// Regenerate enumerates once and rewrites the CDI spec. Returns the device
// count so callers can log it.
func (m *Monitor) Regenerate() (int, error) {
	devs, err := listFunc()
	if err != nil {
		return 0, err
	}
	spec := cdi.Generate(devs, m.Libs)
	if err := writeFunc(spec, m.CDIDir); err != nil {
		return 0, err
	}
	return len(devs), nil
}

// Run regenerates immediately, then on each Interval tick until ctx is done.
// Enumeration errors are logged and retried on the next tick rather than fatal.
func (m *Monitor) Run(ctx context.Context) {
	if n, err := m.Regenerate(); err != nil {
		log.Printf("initial CDI generation failed: %v", err)
	} else {
		log.Printf("CDI generated for %d device(s)", n)
	}

	ticker := time.NewTicker(m.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if n, err := m.Regenerate(); err != nil {
				log.Printf("CDI regeneration failed: %v", err)
			} else {
				log.Printf("CDI refreshed for %d device(s)", n)
			}
		case <-ctx.Done():
			return
		}
	}
}
