# dx-k8s-device-plugin

Kubernetes device plugin for DEEPX DX-M1 NPUs. Advertises `deepx.ai/dx-m1`, injects
NPU device nodes into pods via CDI. Kernel driver + firmware are a **host
prerequisite** (installed by `dx-runtime/install.sh`); this plugin only discovers,
health-checks, and schedules the cards.

Consumed by [`dx-all-suite`](https://github.com/DEEPX-AI/dx-all-suite) as a
submodule; deployed via the `dx-npu` Helm chart there.

## Components

- **`internal/dxdevice`** — enumeration + health. sysfs (`/sys/class/dxrt/dxrtN`) is
  authoritative for the allocatable card list; `dxcli -s` supplies metadata
  (product, RT/PCIe driver, firmware, PCIe BDF) and health.
- **`internal/cdi`** — CDI 0.6.0 spec generation (`/etc/cdi/deepx.json`): one CDI
  device per card, plus host runtime-lib and `dxcli` mounts so app images stay thin.
- **`internal/plugin`** — kubelet Device Plugin API: `ListAndWatch` (health-aware)
  and `Allocate` (CDI dual-path: typed `CDIDevices` + legacy annotation).
- **`internal/nfd`** — node-feature-discovery local feature file, turned by the NFD
  worker into node labels for card count, product, and firmware/driver versions.
- **`internal/metrics`** — `deepx_npu_*` Prometheus gauges (device health, per-core
  temperature/voltage/clock), served on `METRICS_ADDR`.
- **`internal/monitor` + `cmd/dx-device-plugin`** — regeneration loop, gRPC server,
  kubelet registration, re-register on kubelet restart (fsnotify).
- **Dockerfile + CI + `deploy/`** — multi-arch image to ghcr, raw DaemonSet and
  smoke-test pod. The runtime base image needs a glibc at least as new as the
  host's DXRT build, or every card reports Unhealthy.

## Deploy (dev)

```bash
# each NPU node, host: driver + firmware + runtime
cd dx-runtime && ./install.sh --runtime-only
# enable CDI in k3s containerd (enable_cdi=true, cdi_spec_dirs incl /etc/cdi), then:
kubectl apply -f deploy/dx-device-plugin.yaml
kubectl apply -f deploy/test-pod.yaml && kubectl logs dx-m1-test
```

## Facts (DX-M1, verified on hardware)

| | |
|---|---|
| PCI vendor:device | `1ff4:0100` |
| Device node | `/dev/dxrtN` (one per card, char major 507) |
| Enumeration | `ls /sys/class/dxrt/` |
| Metadata/health | `dxcli -s [-d N]` |
| Resource | `deepx.ai/dx-m1` (whole-device) |

## Test

```bash
go test ./...
```

`TestList_RealHardware` runs the full sysfs+dxcli path on a node with an NPU and
skips automatically where none is present.
