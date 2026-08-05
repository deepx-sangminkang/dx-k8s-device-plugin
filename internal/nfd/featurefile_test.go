package nfd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

func TestWrite_LabelsFromDevices(t *testing.T) {
	dir := t.TempDir()
	devs := []dxdevice.Device{
		{ID: 0, FWVersion: "v2.7.3", RTDriver: "v2.5.1", PCIeDriver: "v2.4.1", Product: "M1", Healthy: true},
		{ID: 1, FWVersion: "v2.7.3", RTDriver: "v2.5.1", PCIeDriver: "v2.4.1", Product: "M1", Healthy: true},
	}
	if err := Write(devs, dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		"deepx.ai/dx-m1.count=2\n",
		"deepx.ai/dx-m1.product=M1\n",
		"deepx.ai/dx-m1.fw-version=v2.7.3\n",
		"deepx.ai/dx-m1.driver-version=v2.5.1\n",
		"deepx.ai/dx-m1.pcie-driver-version=v2.4.1\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("feature file missing %q\n---\n%s", want, got)
		}
	}
}

func TestWrite_NoDevices(t *testing.T) {
	dir := t.TempDir()
	if err := Write(nil, dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "deepx.ai/dx-m1.count=0\n" {
		t.Errorf("zero-device file should only carry count=0, got:\n%s", got)
	}
}
