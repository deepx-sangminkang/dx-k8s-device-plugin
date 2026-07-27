package plugin

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// registerRetries is how many times registration is attempted before giving up.
var registerRetries = 5

// registerWithKubelet dials the kubelet registration socket and registers this
// plugin's endpoint + resource name, retrying with linear backoff.
func registerWithKubelet(ctx context.Context, kubeletSocket, endpoint, resourceName string) error {
	req := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     endpoint, // basename of our socket under the device-plugins dir
		ResourceName: resourceName,
	}

	var lastErr error
	for attempt := 1; attempt <= registerRetries; attempt++ {
		if err := registerOnce(ctx, kubeletSocket, req); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	return fmt.Errorf("kubelet registration failed after %d attempts: %w", registerRetries, lastErr)
}

func registerOnce(ctx context.Context, kubeletSocket string, req *pluginapi.RegisterRequest) error {
	conn, err := grpc.NewClient("unix:"+kubeletSocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial kubelet: %w", err)
	}
	defer conn.Close()

	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := pluginapi.NewRegistrationClient(conn).Register(rctx, req); err != nil {
		return fmt.Errorf("register RPC: %w", err)
	}
	return nil
}
