# syntax=docker/dockerfile:1
# Multi-arch build. The binary is static (CGO off); dxrt-cli and libdxrt are NOT
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

# debian-slim (not distroless): the plugin shells out to the host's dynamically
# linked dxrt-cli for metadata/health, so it needs a libc/libstdc++ present. The
# dxrt-cli binary + libdxrt/libonnxruntime are mounted from the host at runtime.
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/dx-device-plugin /usr/bin/dx-device-plugin
ENTRYPOINT ["/usr/bin/dx-device-plugin"]
