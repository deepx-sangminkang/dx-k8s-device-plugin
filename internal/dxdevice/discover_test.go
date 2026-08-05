package dxdevice

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func TestParseStatus_OneDevice(t *testing.T) {
	devs := parseStatus(readFixture(t, "status-1dev.txt"))
	if len(devs) != 1 {
		t.Fatalf("want 1 device, got %d", len(devs))
	}
	d := devs[0]
	want := Device{
		ID: 0, Name: "dxrt0", NodePath: "/dev/dxrt0",
		Product: "M1", RTDriver: "v2.5.1", PCIeDriver: "v2.4.1",
		FWVersion: "v2.7.3", Board: "M.2, Rev 1.0", PCIe: "Gen3 X4 [85:00:00]",
		Cores: []Core{
			{ID: 0, TemperatureC: 43, VoltageMV: 750, ClockMHz: 1000},
			{ID: 1, TemperatureC: 42, VoltageMV: 750, ClockMHz: 1000},
			{ID: 2, TemperatureC: 43, VoltageMV: 750, ClockMHz: 1000},
		},
		Healthy: true,
	}
	if !reflect.DeepEqual(d, want) {
		t.Errorf("device 0 mismatch:\n got  %+v\n want %+v", d, want)
	}
}

// Per-core stats come from the `NPU N: voltage .. mV, clock .. MHz,
// temperature ..'C` line in each device block.
func TestParseStatus_Cores(t *testing.T) {
	devs := parseStatus(readFixture(t, "status-2dev.txt"))
	want0 := []Core{{ID: 0, TemperatureC: 43, VoltageMV: 750, ClockMHz: 1000}}
	want1 := []Core{{ID: 0, TemperatureC: 41, VoltageMV: 750, ClockMHz: 1000}}
	if !reflect.DeepEqual(devs[0].Cores, want0) {
		t.Errorf("device 0 cores = %+v, want %+v", devs[0].Cores, want0)
	}
	if !reflect.DeepEqual(devs[1].Cores, want1) {
		t.Errorf("device 1 cores = %+v, want %+v", devs[1].Cores, want1)
	}
}

func TestParseStatus_TwoDevices(t *testing.T) {
	devs := parseStatus(readFixture(t, "status-2dev.txt"))
	if len(devs) != 2 {
		t.Fatalf("want 2 devices, got %d", len(devs))
	}
	// Colon-bearing BDF must survive the key:value split intact, per device.
	if got := devs[1].PCIe; got != "Gen3 X4 [86:00:00]" {
		t.Errorf("device 1 PCIe = %q, want %q", got, "Gen3 X4 [86:00:00]")
	}
	if devs[0].PCIe != "Gen3 X4 [85:00:00]" {
		t.Errorf("device 0 PCIe = %q", devs[0].PCIe)
	}
}

func TestParseStatus_Empty(t *testing.T) {
	if devs := parseStatus(""); len(devs) != 0 {
		t.Errorf("empty input should yield 0 devices, got %d", len(devs))
	}
}

// List merges sysfs enumeration (authoritative) with dxrt-cli metadata, and
// marks a sysfs-present-but-status-absent card Unhealthy.
func TestList_MergesSysfsAndStatus(t *testing.T) {
	dir := t.TempDir()
	// Fake sysfs: dxrt0 (in status) and dxrt1 (NOT in status → wedged).
	for _, n := range []string{"dxrt0", "dxrt1", "not-a-device"} {
		if err := os.Mkdir(filepath.Join(dir, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	swap(t, &sysClassDxrt, dir)
	swap(t, &statusCmd, func(id int) (string, error) {
		return readFixture(t, "status-1dev.txt"), nil // reports only device 0
	})

	devs, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(devs) != 2 {
		t.Fatalf("want 2 sysfs cards (dxrt0, dxrt1), got %d: %+v", len(devs), devs)
	}
	if !devs[0].Healthy || devs[0].FWVersion != "v2.7.3" {
		t.Errorf("dxrt0 should be healthy+enriched, got %+v", devs[0])
	}
	if len(devs[0].Cores) != 3 {
		t.Errorf("dxrt0 cores not merged from status, got %+v", devs[0].Cores)
	}
	if devs[1].Healthy {
		t.Errorf("dxrt1 absent from status must be Unhealthy, got %+v", devs[1])
	}
}

func TestHealth(t *testing.T) {
	swap(t, &statusCmd, func(id int) (string, error) {
		return readFixture(t, "status-1dev.txt"), nil
	})
	if !Health(0) {
		t.Error("device 0 present in status should be Healthy")
	}
	if Health(5) {
		t.Error("device 5 absent from status should be Unhealthy")
	}
}

// TestList_RealHardware exercises the full path (sysfs read + real dxrt-cli exec
// + parse) on a node that actually has a DX-M1. It skips cleanly on CI / dev
// machines without an NPU, so it never blocks the suite.
func TestList_RealHardware(t *testing.T) {
	if _, err := os.Stat(sysClassDxrt); err != nil {
		t.Skipf("no %s on this host, skipping real-hardware test", sysClassDxrt)
	}
	// Containers see the host's /sys but usually lack the DEEPX userland;
	// without dxrt-cli every card parses as Unhealthy, which is an environment
	// gap, not a plugin bug.
	if _, err := exec.LookPath("dxrt-cli"); err != nil {
		t.Skip("dxrt-cli not in PATH, skipping real-hardware test")
	}
	devs, err := List()
	if err != nil {
		t.Fatalf("List on real hardware: %v", err)
	}
	if len(devs) == 0 {
		t.Skip("sysfs class present but no dxrtN cards enumerated")
	}
	for _, d := range devs {
		t.Logf("found %s node=%s product=%s fw=%s pcie=%q healthy=%v",
			d.Name, d.NodePath, d.Product, d.FWVersion, d.PCIe, d.Healthy)
		if !d.Healthy {
			t.Errorf("%s reported Unhealthy on a live node", d.Name)
		}
		if d.FWVersion == "" {
			t.Errorf("%s has empty FWVersion — dxrt-cli parse likely broke", d.Name)
		}
	}
}

// swap temporarily replaces *p with v and restores it after the test.
func swap[T any](t *testing.T, p *T, v T) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}
