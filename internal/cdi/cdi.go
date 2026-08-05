// Package cdi turns a list of discovered DX-M1 cards into a Container Device
// Interface (CDI) spec that containerd/CRI-O consume to inject NPU device nodes
// (and, optionally, the host runtime libraries) into pods.
//
// CDI is the injection mechanism all mature NPU/GPU vendors converged on; using
// it here means app images need not bake dx_rt — the host libs are mounted in.
// The device-plugin Allocate step references these devices by name
// (`deepx.ai/dx-m1=<id>`); the kubelet/containerd then applies the containerEdits
// from this spec.
package cdi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

// Kind is the CDI kind / device-plugin resource name for DEEPX DX-M1 NPUs.
const Kind = "deepx.ai/dx-m1"

// specVersion is the CDI schema version we emit. 0.6.0 is widely supported by
// containerd 1.7+/2.x (the versions relevant for k3s CDI).
const specVersion = "0.6.0"

// DefaultLibs are the host runtime shared objects mounted into every NPU pod so
// that thin app images (no dx_rt baked in) can still run inference. Paths are
// the verified DX-M1 install locations; override via Generate's libs argument
// (pass nil to emit a device-node-only spec for images that bundle dx_rt).
var DefaultLibs = []string{
	"/usr/local/lib/libdxrt.so.3",
	"/usr/local/lib/libonnxruntime.so.1",
	// dxrt-cli rides along for in-pod diagnostics, the way nvidia-ctk
	// injects nvidia-smi.
	"/usr/local/bin/dxrt-cli",
}

// Spec is the subset of the CDI schema we produce.
type Spec struct {
	Version        string            `json:"cdiVersion"`
	Kind           string            `json:"kind"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Devices        []Device          `json:"devices"`
	ContainerEdits *ContainerEdits   `json:"containerEdits,omitempty"`
}

type Device struct {
	Name           string            `json:"name"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	ContainerEdits ContainerEdits    `json:"containerEdits"`
}

type ContainerEdits struct {
	DeviceNodes []DeviceNode `json:"deviceNodes,omitempty"`
	Mounts      []Mount      `json:"mounts,omitempty"`
}

// DeviceNode omits Major/Minor on purpose: the CDI runtime resolves them by
// stat-ing HostPath, so we never hardcode the driver's dynamically-allocated
// char major (507 today, but not guaranteed across boots/kernels).
type DeviceNode struct {
	Path        string `json:"path"`
	HostPath    string `json:"hostPath,omitempty"`
	Type        string `json:"type,omitempty"`
	Permissions string `json:"permissions,omitempty"`
}

type Mount struct {
	HostPath      string   `json:"hostPath"`
	ContainerPath string   `json:"containerPath"`
	Options       []string `json:"options,omitempty"`
}

// Generate builds a CDI spec: one CDI device per card (named by numeric ID, so
// Allocate can reference "deepx.ai/dx-m1=<id>"), each injecting its /dev/dxrtN
// node. Any libs are added as global mounts applied to every device.
func Generate(devs []dxdevice.Device, libs []string) Spec {
	spec := Spec{
		Version: specVersion,
		Kind:    Kind,
		Annotations: map[string]string{
			"vendor": "DEEPX Co., Ltd.",
		},
		Devices: make([]Device, 0, len(devs)),
	}

	for _, d := range devs {
		spec.Devices = append(spec.Devices, Device{
			Name: strconv.Itoa(d.ID),
			Annotations: map[string]string{
				"device.model": d.Product,
				"device.pcie":  d.PCIe,
			},
			ContainerEdits: ContainerEdits{
				DeviceNodes: []DeviceNode{{
					Path:        d.NodePath,
					HostPath:    d.NodePath,
					Type:        "c",
					Permissions: "rw",
				}},
			},
		})
	}

	if len(libs) > 0 {
		mounts := make([]Mount, 0, len(libs))
		for _, l := range libs {
			mounts = append(mounts, Mount{
				HostPath:      l,
				ContainerPath: l,
				Options:       []string{"ro", "bind"},
			})
		}
		spec.ContainerEdits = &ContainerEdits{Mounts: mounts}
	}

	return spec
}

// Write marshals spec to <dir>/deepx.json atomically (write temp + rename) so a
// concurrent reader never sees a half-written file. dir is typically /etc/cdi.
func Write(spec Spec, dir string) error {
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	final := filepath.Join(dir, "deepx.json")
	tmp, err := os.CreateTemp(dir, ".deepx-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, final); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename %s -> %s: %w", tmpName, final, err)
	}
	return nil
}
