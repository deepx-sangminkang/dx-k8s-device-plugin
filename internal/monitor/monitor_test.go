package monitor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/cdi"
	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

func swap[T any](t *testing.T, p *T, v T) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

func TestRegenerate_WritesSpec(t *testing.T) {
	swap(t, &listFunc, func() ([]dxdevice.Device, error) {
		return []dxdevice.Device{
			{ID: 0, Name: "dxrt0", NodePath: "/dev/dxrt0", Product: "M1", Healthy: true},
		}, nil
	})
	dir := t.TempDir()
	m := &Monitor{CDIDir: dir, Libs: cdi.DefaultLibs}

	n, err := m.Regenerate()
	if err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
	if n != 1 {
		t.Errorf("device count = %d, want 1", n)
	}

	data, err := os.ReadFile(filepath.Join(dir, "deepx.json"))
	if err != nil {
		t.Fatalf("deepx.json not written: %v", err)
	}
	var spec cdi.Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatalf("invalid spec: %v", err)
	}
	if spec.Kind != cdi.Kind || len(spec.Devices) != 1 {
		t.Errorf("spec kind=%q devices=%d", spec.Kind, len(spec.Devices))
	}
}

func TestRegenerate_EnumErrorPropagates(t *testing.T) {
	swap(t, &listFunc, func() ([]dxdevice.Device, error) {
		return nil, errors.New("boom")
	})
	m := &Monitor{CDIDir: t.TempDir()}
	if _, err := m.Regenerate(); err == nil {
		t.Error("expected enumeration error to propagate")
	}
}
