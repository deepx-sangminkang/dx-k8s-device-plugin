// Command dx-device-plugin is the DEEPX DX-M1 Kubernetes device plugin entrypoint.
//
// It runs two loops until signalled:
//   - a CDI monitor that keeps /etc/cdi/deepx.json in sync with the NPUs present,
//   - a gRPC device-plugin server registered with kubelet, re-registering
//     whenever kubelet restarts (detected by its socket being recreated).
package main

import (
	"context"
	"log"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/cdi"
	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/monitor"
	"github.com/deepx-sangminkang/dx-k8s-device-plugin/internal/plugin"
)

const (
	pluginSocket = pluginDir + "/dx-m1.sock"
	pluginDir    = "/var/lib/kubelet/device-plugins"
	cdiDir       = "/etc/cdi"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("[dx-device-plugin] ")
	log.Println("starting")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// CDI monitor: keep /etc/cdi/deepx.json current for containerd.
	mon := &monitor.Monitor{CDIDir: cdiDir, Libs: cdi.DefaultLibs, Interval: 60 * time.Second}
	go mon.Run(ctx)

	// Device-plugin server, (re)registered with kubelet.
	if err := serveAndRegister(ctx); err != nil {
		log.Fatalf("initial serve/register: %v", err)
	}

	if err := watchKubelet(ctx); err != nil {
		log.Printf("kubelet watch ended: %v", err)
	}
	log.Println("shutting down")
}

// current holds the running server so the watcher can restart it.
var current *plugin.Server

func serveAndRegister(ctx context.Context) error {
	if current != nil {
		current.Stop()
	}
	srv := plugin.NewServer(plugin.New(), pluginSocket)
	if err := srv.Serve(); err != nil {
		return err
	}
	if err := srv.Register(ctx); err != nil {
		srv.Stop()
		return err
	}
	current = srv
	log.Printf("registered %s with kubelet", srv.ResourceName)
	return nil
}

// watchKubelet re-registers when kubelet recreates its socket (kubelet wipes
// plugin registrations on restart, so we must re-announce).
func watchKubelet(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()
	if err := w.Add(pluginDir); err != nil {
		return err
	}
	kubeletSock := filepath.Join(pluginDir, "kubelet.sock")

	for {
		select {
		case ev := <-w.Events:
			if ev.Name == kubeletSock && ev.Op&(fsnotify.Create) != 0 {
				log.Println("kubelet socket recreated, re-registering")
				if err := serveAndRegister(ctx); err != nil {
					log.Printf("re-register failed: %v", err)
				}
			}
		case err := <-w.Errors:
			log.Printf("watcher error: %v", err)
		case <-ctx.Done():
			if current != nil {
				current.Stop()
			}
			return ctx.Err()
		}
	}
}
