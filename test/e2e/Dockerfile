FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev libpcap-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build xbslink-ng
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /usr/local/bin/xbslink-ng ./cmd/xbslink-ng

# Build xbox-sim
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /usr/local/bin/xbox-sim ./test/e2e/xbox-sim

# Runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache libpcap bash

# Copy binaries
COPY --from=builder /usr/local/bin/xbslink-ng /usr/local/bin/
COPY --from=builder /usr/local/bin/xbox-sim /usr/local/bin/

# Default command
CMD ["xbslink-ng", "version"]
