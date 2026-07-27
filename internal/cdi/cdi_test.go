package cdi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

func sampleDevices() []dxdevice.Device {
	return []dxdevice.Device{
		{ID: 0, Name: "dxrt0", NodePath: "/dev/dxrt0", Product: "M1", PCIe: "Gen3 X4 [85:00:00]", Healthy: true},
		{ID: 1, Name: "dxrt1", NodePath: "/dev/dxrt1", Product: "M1", PCIe: "Gen3 X4 [86:00:00]", Healthy: true},
	}
}

func TestGenerate_DeviceNodes(t *testing.T) {
	spec := Generate(sampleDevices(), nil)

	if spec.Version != specVersion || spec.Kind != Kind {
		t.Fatalf("bad header: version=%q kind=%q", spec.Version, spec.Kind)
	}
	if len(spec.Devices) != 2 {
		t.Fatalf("want 2 CDI devices, got %d", len(spec.Devices))
	}
	// Names must be the numeric IDs so Allocate can ref "deepx.ai/npu=<id>".
	if spec.Devices[0].Name != "0" || spec.Devices[1].Name != "1" {
		t.Errorf("device names = %q,%q want 0,1", spec.Devices[0].Name, spec.Devices[1].Name)
	}
	dn := spec.Devices[1].ContainerEdits.DeviceNodes
	if len(dn) != 1 || dn[0].Path != "/dev/dxrt1" || dn[0].HostPath != "/dev/dxrt1" {
		t.Errorf("device 1 node wrong: %+v", dn)
	}
	if dn[0].Type != "c" || dn[0].Permissions != "rw" {
		t.Errorf("device node type/perm wrong: %+v", dn[0])
	}
	// nil libs → no global mounts.
	if spec.ContainerEdits != nil {
		t.Errorf("expected no global containerEdits with nil libs, got %+v", spec.ContainerEdits)
	}
}

func TestGenerate_WithLibs(t *testing.T) {
	spec := Generate(sampleDevices(), DefaultLibs)
	if spec.ContainerEdits == nil || len(spec.ContainerEdits.Mounts) != len(DefaultLibs) {
		t.Fatalf("want %d global lib mounts, got %+v", len(DefaultLibs), spec.ContainerEdits)
	}
	m := spec.ContainerEdits.Mounts[0]
	if m.HostPath != DefaultLibs[0] || m.ContainerPath != DefaultLibs[0] {
		t.Errorf("lib mount path wrong: %+v", m)
	}
	if len(m.Options) != 2 || m.Options[0] != "ro" {
		t.Errorf("lib mount should be ro,bind: %+v", m.Options)
	}
}

func TestGenerate_Empty(t *testing.T) {
	spec := Generate(nil, DefaultLibs)
	if len(spec.Devices) != 0 {
		t.Errorf("no devices should yield empty Devices, got %d", len(spec.Devices))
	}
	// devices is a non-nil empty slice → marshals to [] not null (valid CDI).
	b, _ := json.Marshal(spec)
	if !contains(string(b), `"devices":[]`) {
		t.Errorf("devices should marshal to []: %s", b)
	}
}

func TestWrite_AtomicAndReadable(t *testing.T) {
	dir := t.TempDir()
	spec := Generate(sampleDevices(), DefaultLibs)
	if err := Write(spec, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Exactly deepx.json exists (no leftover temp files).
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "deepx.json" {
		t.Fatalf("want only deepx.json, got %v", names(entries))
	}

	// Round-trips back to an equivalent spec.
	data, err := os.ReadFile(filepath.Join(dir, "deepx.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got Spec
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("written spec is not valid JSON: %v", err)
	}
	if got.Kind != Kind || len(got.Devices) != 2 {
		t.Errorf("round-trip mismatch: kind=%q devices=%d", got.Kind, len(got.Devices))
	}
}

// TestGenerate_RealHardware generates a CDI spec from the live NPU(s) on this
// node and validates it round-trips. Skips where no DX-M1 is present.
func TestGenerate_RealHardware(t *testing.T) {
	if _, err := os.Stat("/sys/class/dxrt"); err != nil {
		t.Skip("no DX-M1 on this host")
	}
	devs, err := dxdevice.List()
	if err != nil || len(devs) == 0 {
		t.Skipf("no cards enumerated (err=%v)", err)
	}
	spec := Generate(devs, DefaultLibs)
	if len(spec.Devices) != len(devs) {
		t.Fatalf("spec has %d devices, want %d", len(spec.Devices), len(devs))
	}
	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	t.Logf("live CDI spec:\n%s", b)
	if spec.Devices[0].ContainerEdits.DeviceNodes[0].HostPath != devs[0].NodePath {
		t.Errorf("device node hostPath mismatch")
	}
}

func names(es []os.DirEntry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
