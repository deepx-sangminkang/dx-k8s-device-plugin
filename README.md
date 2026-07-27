# dx-k8s-device-plugin

Kubernetes device plugin for DEEPX DX-M1 NPUs. Advertises `deepx.ai/dx-m1`, injects
NPU device nodes into pods via CDI. Kernel driver + firmware are a **host
prerequisite** (installed by `dx-runtime/install.sh`); this plugin only discovers,
health-checks, and schedules the cards.

Consumed by [`dx-all-suite`](https://github.com/deepx-sangminkang/dx-all-suite) as a
submodule; deployed via the `dx-npu` Helm chart there.

## Status

Core plugin complete; suite-side Helm chart / NFD / metrics still pending.

- **`internal/dxdevice`** (P1) — shared enumeration + health. sysfs
  (`/sys/class/dxrt/dxrtN`) is authoritative for the allocatable card list;
  `dxrt-cli -s` supplies metadata (product, RT/PCIe driver, firmware, PCIe BDF)
  and health.
- **`internal/cdi`** (P2) — CDI 0.6.0 spec generation (`/etc/cdi/deepx.json`):
  one CDI device per card + optional host runtime-lib mounts for thin images.
- **`internal/plugin`** (P3) — kubelet Device Plugin API: `ListAndWatch`
  (health-aware) + `Allocate` (CDI dual-path: typed `CDIDevices` + legacy annotation).
- **`internal/monitor` + `cmd/dx-device-plugin`** (P4) — CDI regen loop, gRPC
  server, kubelet registration, re-register on kubelet restart (fsnotify).
- **Dockerfile + CI + `deploy/`** (P5) — multi-arch image → ghcr, raw DaemonSet
  and smoke-test pod.

Planned (in `dx-all-suite`): `dx-npu` Helm chart, NFD rule (PCI `1ff4` → labels),
optional `deepx_npu_*` metrics exporter.

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
| Metadata/health | `dxrt-cli -s [-d N]` |
| Resource | `deepx.ai/dx-m1` (whole-device) |

## Test

```bash
go test ./...
```

`TestList_RealHardware` runs the full sysfs+dxrt-cli path on a node with an NPU and
skips automatically where none is present.
