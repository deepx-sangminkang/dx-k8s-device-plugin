# dx-k8s-device-plugin

Kubernetes device plugin for DEEPX DX-M1 NPUs. Advertises `deepx.ai/dx-m1`, injects
NPU device nodes into pods via CDI. Kernel driver + firmware are a **host
prerequisite** (installed by `dx-runtime/install.sh`); this plugin only discovers,
health-checks, and schedules the cards.

Consumed by [`dx-all-suite`](https://github.com/deepx-sangminkang/dx-all-suite) as a
submodule; deployed via the `dx-npu` Helm chart there.

## Status

Under construction. Implemented so far:

- **`internal/dxdevice`** (P1) — shared enumeration + health package. sysfs
  (`/sys/class/dxrt/dxrtN`) is authoritative for the allocatable card list;
  `dxrt-cli -s` supplies metadata (product, RT/PCIe driver, firmware, PCIe BDF)
  and health.

Planned: CDI generator (P2), device-plugin gRPC server (P3), lifecycle +
kubelet-restart handling (P4), multi-arch image + CI (P5).

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
