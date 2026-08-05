// Package nfd publishes DX-M1 metadata as a node-feature-discovery "local"
// feature file, which the NFD worker turns into node labels — fw/driver
// version labels without an extra DaemonSet or RBAC (the device plugin
// already runs on every NPU node).
//
// NFD's local source accepts explicitly-prefixed label names (deepx.ai/...)
// as key=value lines. The target dir is NFD's features.d, hostPath-mounted by
// the dx-npu chart.
package nfd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

// FileName is our feature file under NFD's features.d directory.
const FileName = "deepx"

// Write renders the feature file atomically (temp+rename, same pattern as
// cdi.Write). Version/product labels come from the first device — cards in
// one host share driver/fw in practice.
func Write(devs []dxdevice.Device, dir string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "deepx.ai/dx-m1.count=%d\n", len(devs))
	if len(devs) > 0 {
		d := devs[0]
		fmt.Fprintf(&b, "deepx.ai/dx-m1.product=%s\n", d.Product)
		fmt.Fprintf(&b, "deepx.ai/dx-m1.fw-version=%s\n", d.FWVersion)
		fmt.Fprintf(&b, "deepx.ai/dx-m1.driver-version=%s\n", d.RTDriver)
		fmt.Fprintf(&b, "deepx.ai/dx-m1.pcie-driver-version=%s\n", d.PCIeDriver)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".deepx-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, filepath.Join(dir, FileName))
}
