// Package dxdevice enumerates and health-checks DEEPX DX-M1 NPU devices on a host.
//
// It is the shared keystone package for the device plugin, CDI generator, and
// (later) metrics exporter: everything that needs to know "which NPUs are on
// this node and are they healthy" goes through here.
//
// Enumeration source of truth is sysfs (/sys/class/dxrt/dxrtN) — one entry per
// card, which is the whole-device allocation unit advertised as deepx.ai/npu.
// Metadata (product, driver/firmware versions, PCIe BDF) and health come from
// parsing `dxrt-cli -s`.
package dxdevice

// Device is one allocatable DX-M1 NPU card (one /dev/dxrtN node).
//
// A single card contains multiple internal NPU cores (dxrt-cli reports "NPU 0/1/2")
// but they are not independently schedulable — the card is allocated whole.
type Device struct {
	ID       int    // sysfs index, e.g. 0 for dxrt0
	Name     string // "dxrt0"
	NodePath string // "/dev/dxrt0"

	Product    string // "M1"
	RTDriver   string // runtime driver version, e.g. "v2.5.1"
	PCIeDriver string // PCIe driver version, e.g. "v2.4.1"
	FWVersion  string // firmware version, e.g. "v2.7.3"
	Board      string // "M.2, Rev 1.0"
	PCIe       string // "Gen3 X4 [85:00:00]" (BDF used as stable identity)

	// Healthy is true when the card is present in sysfs AND dxrt-cli reports a
	// status block for it. A card that exists in sysfs but is missing from
	// dxrt-cli output (wedged/recovering) is reported Unhealthy so the device
	// plugin stops scheduling onto it.
	Healthy bool
}
