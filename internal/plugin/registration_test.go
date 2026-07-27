package plugin

import (
	"context"
	"testing"
	"time"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// With no kubelet listening, registration must exhaust retries and return an
// error rather than hang — and respect context cancellation promptly.
func TestRegister_FailsFastWithoutKubelet(t *testing.T) {
	swap(t, &registerRetries, 2)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := registerWithKubelet(ctx, "/nonexistent/kubelet.sock", "dx-m1.sock", "deepx.ai/dx-m1")
	if err == nil {
		t.Fatal("expected registration to fail without a kubelet")
	}
}

func TestRegister_CanceledContext(t *testing.T) {
	swap(t, &registerRetries, 100)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	done := make(chan error, 1)
	go func() {
		done <- registerWithKubelet(ctx, "/nonexistent/kubelet.sock", "dx-m1.sock", "deepx.ai/dx-m1")
	}()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("registration did not honor canceled context")
	}
}

func TestServer_NewSetsFields(t *testing.T) {
	s := NewServer(New(), "/tmp/dx-m1.sock")
	if s.ResourceName != "deepx.ai/dx-m1" || s.SocketPath != "/tmp/dx-m1.sock" {
		t.Errorf("server fields wrong: %+v", s)
	}
	// Sanity: the kubelet socket constant is the documented well-known path.
	if KubeletSocket != pluginapi.KubeletSocket {
		t.Errorf("KubeletSocket = %q", KubeletSocket)
	}
}
