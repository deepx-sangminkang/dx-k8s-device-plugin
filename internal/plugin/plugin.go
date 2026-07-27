// Package plugin implements the kubelet Device Plugin API for DEEPX DX-M1 NPUs.
//
// It advertises each card as one unit of the extended resource deepx.ai/dx-m1,
// reports per-card health from dxdevice, and on Allocate hands the container a
// CDI device reference (deepx.ai/dx-m1=<id>) that containerd resolves against
// the spec written by the CDI generator.
package plugin

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/cdi"
	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

// listFunc is the enumeration seam (overridden in tests). Defaults to the real host.
var listFunc = dxdevice.List

// watchInterval is how often ListAndWatch re-enumerates to catch device
// health/plug changes. Overridable in tests.
var watchInterval = 30 * time.Second

// DevicePlugin implements pluginapi.DevicePluginServer.
type DevicePlugin struct {
	// ResourceName is the advertised extended resource, e.g. deepx.ai/dx-m1.
	ResourceName string
}

var _ pluginapi.DevicePluginServer = (*DevicePlugin)(nil)

// New returns a plugin advertising cdi.Kind (deepx.ai/dx-m1).
func New() *DevicePlugin {
	return &DevicePlugin{ResourceName: cdi.Kind}
}

func (p *DevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

// ListAndWatch streams the current device list to kubelet, then re-sends on each
// tick so health transitions (a card going wedged) propagate.
func (p *DevicePlugin) ListAndWatch(_ *pluginapi.Empty, srv pluginapi.DevicePlugin_ListAndWatchServer) error {
	if err := p.send(srv); err != nil {
		return err
	}
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := p.send(srv); err != nil {
				log.Printf("ListAndWatch send failed: %v", err)
				return err
			}
		case <-srv.Context().Done():
			return srv.Context().Err()
		}
	}
}

func (p *DevicePlugin) send(srv pluginapi.DevicePlugin_ListAndWatchServer) error {
	devs, err := listFunc()
	if err != nil {
		log.Printf("device enumeration failed: %v", err)
		devs = nil // report empty rather than crash the stream
	}
	return srv.Send(&pluginapi.ListAndWatchResponse{Devices: toAPIDevices(devs)})
}

// toAPIDevices maps discovered cards to kubelet device entries, translating
// dxdevice health into the kubelet Healthy/Unhealthy constants.
func toAPIDevices(devs []dxdevice.Device) []*pluginapi.Device {
	out := make([]*pluginapi.Device, 0, len(devs))
	for _, d := range devs {
		health := pluginapi.Unhealthy
		if d.Healthy {
			health = pluginapi.Healthy
		}
		out = append(out, &pluginapi.Device{ID: strconv.Itoa(d.ID), Health: health})
	}
	return out
}

// Allocate returns, per container, the CDI device references for the requested
// card IDs. It populates BOTH the typed CDIDevices field (kubelet v1.27+ with
// containerd 1.7+/2.x) and the legacy cdi.k8s.io/* annotation (older
// toolchains) — modern stacks read the former and ignore the latter, older
// ones do the reverse, so emitting both works across versions.
func (p *DevicePlugin) Allocate(_ context.Context, req *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	resp := &pluginapi.AllocateResponse{}
	for _, cr := range req.ContainerRequests {
		resp.ContainerResponses = append(resp.ContainerResponses, p.allocateContainer(cr.DevicesIDs))
	}
	return resp, nil
}

func (p *DevicePlugin) allocateContainer(ids []string) *pluginapi.ContainerAllocateResponse {
	names := make([]string, 0, len(ids))
	cdiDevs := make([]*pluginapi.CDIDevice, 0, len(ids))
	for _, id := range ids {
		name := fmt.Sprintf("%s=%s", p.ResourceName, id) // deepx.ai/dx-m1=<id>
		names = append(names, name)
		cdiDevs = append(cdiDevs, &pluginapi.CDIDevice{Name: name})
	}
	return &pluginapi.ContainerAllocateResponse{
		Annotations: map[string]string{"cdi.k8s.io/dx-m1": joinCSV(names)},
		CDIDevices:  cdiDevs,
	}
}

func joinCSV(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v
	}
	return out
}

func (p *DevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (p *DevicePlugin) GetPreferredAllocation(context.Context, *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return &pluginapi.PreferredAllocationResponse{}, nil
}
