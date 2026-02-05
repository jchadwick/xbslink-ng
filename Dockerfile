# Multi-stage build for xbslink-ng
# Stage 1: Build the Go binary
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make libpcap-dev gcc musl-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with version info
ARG VERSION=dev
RUN go build \
    -ldflags="-s -w -X main.Version=${VERSION}" \
    -o xbslink-ng \
    ./cmd/xbslink-ng

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache libpcap

# Copy binary from builder
COPY --from=builder /build/xbslink-ng /usr/local/bin/xbslink-ng

# Labels for GitHub Container Registry
LABEL org.opencontainers.image.title="xbslink-ng"
LABEL org.opencontainers.image.description="P2P Xbox System Link bridge"
LABEL org.opencontainers.image.source="https://github.com/xbslink/xbslink-ng"
LABEL org.opencontainers.image.licenses="MIT"

# Set up non-root user (optional, but packet capture requires capabilities anyway)
# For now, run as root since we need CAP_NET_RAW and CAP_NET_ADMIN
USER root

# Default command shows help
ENTRYPOINT ["/usr/local/bin/xbslink-ng"]
CMD ["help"]

# Notes for running:
# Docker requires --net=host to access host network interfaces
# Docker requires --cap-add=NET_RAW and --cap-add=NET_ADMIN for packet capture
#
# Example listen mode:
# docker run --rm --net=host --cap-add=NET_RAW --cap-add=NET_ADMIN \
#   ghcr.io/xbslink/xbslink-ng:latest \
#   listen --port 31415 --interface eth0 --xbox-mac 00:50:F2:XX:XX:XX
#
# Example connect mode:
# docker run --rm --net=host --cap-add=NET_RAW --cap-add=NET_ADMIN \
#   ghcr.io/xbslink/xbslink-ng:latest \
#   connect --address 1.2.3.4:31415 --interface eth0 --xbox-mac 00:50:F2:YY:YY:YY
