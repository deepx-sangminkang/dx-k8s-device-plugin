package plugin

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// KubeletSocket is the well-known kubelet device-plugin registration socket.
const KubeletSocket = pluginapi.KubeletSocket

// Server owns the plugin's gRPC socket and its registration with kubelet.
type Server struct {
	SocketPath   string // e.g. /var/lib/kubelet/device-plugins/dx-m1.sock
	ResourceName string
	plugin       *DevicePlugin
	grpc         *grpc.Server
}

// NewServer builds a Server serving the given plugin on socketPath.
func NewServer(p *DevicePlugin, socketPath string) *Server {
	return &Server{SocketPath: socketPath, ResourceName: p.ResourceName, plugin: p}
}

// Serve creates the unix socket and starts the gRPC server in the background.
func (s *Server) Serve() error {
	if err := os.Remove(s.SocketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	lis, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.SocketPath, err)
	}
	s.grpc = grpc.NewServer()
	pluginapi.RegisterDevicePluginServer(s.grpc, s.plugin)
	go func() {
		if err := s.grpc.Serve(lis); err != nil {
			log.Printf("gRPC serve stopped: %v", err)
		}
	}()
	return s.waitReady(5 * time.Second)
}

// waitReady dials our own socket until the server answers, so registration
// never races ahead of the listener being live.
func (s *Server) waitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := grpc.NewClient("unix:"+s.SocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			c.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("plugin socket %s not ready within %s", s.SocketPath, timeout)
}

// Stop gracefully stops the gRPC server and removes the socket.
func (s *Server) Stop() {
	if s.grpc != nil {
		s.grpc.GracefulStop()
		s.grpc = nil
	}
	if err := os.Remove(s.SocketPath); err != nil && !os.IsNotExist(err) {
		log.Printf("remove socket: %v", err)
	}
}

// Register announces this plugin to kubelet. Call after Serve.
func (s *Server) Register(ctx context.Context) error {
	return registerWithKubelet(ctx, KubeletSocket, filepath.Base(s.SocketPath), s.ResourceName)
}
