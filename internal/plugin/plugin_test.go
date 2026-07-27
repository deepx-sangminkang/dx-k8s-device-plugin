package plugin

import (
	"context"
	"testing"
	"time"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/cdi"
	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/dxdevice"
)

func TestToAPIDevices_HealthMapping(t *testing.T) {
	devs := []dxdevice.Device{
		{ID: 0, Healthy: true},
		{ID: 1, Healthy: false},
	}
	got := toAPIDevices(devs)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0].ID != "0" || got[0].Health != pluginapi.Healthy {
		t.Errorf("dev0 = %+v, want id=0 Healthy", got[0])
	}
	if got[1].ID != "1" || got[1].Health != pluginapi.Unhealthy {
		t.Errorf("dev1 = %+v, want id=1 Unhealthy", got[1])
	}
}

func TestAllocate_DualPathCDI(t *testing.T) {
	p := New()
	if p.ResourceName != cdi.Kind {
		t.Fatalf("resource name = %q, want %q", p.ResourceName, cdi.Kind)
	}
	if cdi.Kind != "deepx.ai/dx-m1" {
		t.Fatalf("cdi.Kind = %q, want deepx.ai/dx-m1", cdi.Kind)
	}
	resp, err := p.Allocate(context.Background(), &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{DevicesIDs: []string{"0", "1"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.ContainerResponses) != 1 {
		t.Fatalf("want 1 container response, got %d", len(resp.ContainerResponses))
	}
	cr := resp.ContainerResponses[0]

	// Typed CDIDevices field (modern path).
	if len(cr.CDIDevices) != 2 {
		t.Fatalf("want 2 CDIDevices, got %d", len(cr.CDIDevices))
	}
	if cr.CDIDevices[0].Name != "deepx.ai/dx-m1=0" || cr.CDIDevices[1].Name != "deepx.ai/dx-m1=1" {
		t.Errorf("CDIDevice names wrong: %q %q", cr.CDIDevices[0].Name, cr.CDIDevices[1].Name)
	}
	// Legacy annotation path (old kubelet/containerd).
	if got := cr.Annotations["cdi.k8s.io/dx-m1"]; got != "deepx.ai/dx-m1=0,deepx.ai/dx-m1=1" {
		t.Errorf("legacy annotation = %q", got)
	}
}

func TestAllocate_MultiContainer(t *testing.T) {
	p := New()
	resp, _ := p.Allocate(context.Background(), &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{DevicesIDs: []string{"0"}},
			{DevicesIDs: []string{"1"}},
		},
	})
	if len(resp.ContainerResponses) != 2 {
		t.Fatalf("want 2 container responses, got %d", len(resp.ContainerResponses))
	}
	if resp.ContainerResponses[1].CDIDevices[0].Name != "deepx.ai/dx-m1=1" {
		t.Errorf("second container wrong: %+v", resp.ContainerResponses[1].CDIDevices)
	}
}

// fakeLWStream captures ListAndWatchResponses and cancels after a fixed count.
type fakeLWStream struct {
	pluginapi.DevicePlugin_ListAndWatchServer
	ctx    context.Context
	cancel context.CancelFunc
	got    []*pluginapi.ListAndWatchResponse
	max    int
}

func (f *fakeLWStream) Send(r *pluginapi.ListAndWatchResponse) error {
	f.got = append(f.got, r)
	if len(f.got) >= f.max {
		f.cancel()
	}
	return nil
}
func (f *fakeLWStream) Context() context.Context { return f.ctx }

func TestListAndWatch_SendsAndReacts(t *testing.T) {
	// Stub enumeration + fast tick.
	swap(t, &listFunc, func() ([]dxdevice.Device, error) {
		return []dxdevice.Device{{ID: 0, Healthy: true}}, nil
	})
	swap(t, &watchInterval, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	stream := &fakeLWStream{ctx: ctx, cancel: cancel, max: 2}

	err := New().ListAndWatch(&pluginapi.Empty{}, stream)
	if err != context.Canceled {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if len(stream.got) < 2 {
		t.Fatalf("want >=2 sends (initial + tick), got %d", len(stream.got))
	}
	if len(stream.got[0].Devices) != 1 || stream.got[0].Devices[0].ID != "0" {
		t.Errorf("first send wrong: %+v", stream.got[0].Devices)
	}
}

func swap[T any](t *testing.T, p *T, v T) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}
