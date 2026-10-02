# syntax=docker/dockerfile:1
# Multi-arch build. The binary is static (CGO off); dxcli and libdxrt are NOT
# baked in — they are mounted from the host at runtime (see deploy manifest), so
# the plugin always matches the node's installed driver/runtime version.
FROM --platform=$BUILDPLATFORM golang:1.22 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags='-w -s' -o /out/dx-device-plugin ./cmd/dx-device-plugin

# Not distroless: the plugin shells out to the host's dynamically linked dxcli
# for metadata/health, so it needs libc/libstdc++ present. The dxcli binary +
# libdxrt/libonnxruntime are mounted from the host at runtime.
#
# The base glibc must be >= the one the host's dxcli was built against, or every
# card reports Unhealthy. DXRT v3.4.2 needs GLIBC_2.38 / GLIBCXX_3.4.32, which
# debian:bookworm-slim (2.36) lacks; ubuntu:24.04 ships 2.39.
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/dx-device-plugin /usr/bin/dx-device-plugin
ENTRYPOINT ["/usr/bin/dx-device-plugin"]
