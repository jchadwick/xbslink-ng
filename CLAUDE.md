# xbslink-ng

P2P bridge for Xbox System Link traffic over the internet.

## Project Structure

- `cmd/xbslink-ng/` - CLI entrypoint, uses stdlib `flag` (not cobra)
- `internal/bridge/` - Core bridge logic (capture ↔ transport)
- `internal/capture/` - pcap packet capture/injection
- `internal/config/` - Config file management (~/.xbslink-ng/)
- `internal/discovery/` - Xbox MAC auto-discovery via broadcast sniffing
- `internal/events/` - Event emission (JSONLine writer, NopEmitter)
- `internal/logging/` - Leveled logger
- `internal/protocol/` - Wire protocol codec (HELLO, FRAME, PING, PONG, BYE)
- `internal/transport/` - UDP transport (listen/connect modes)
- `xbox-sim/` - Simulated Xbox peer for testing
- `test/testutil/` - Shared test helpers

## Tech Stack

- Go 1.25, module `github.com/xbslink/xbslink-ng`
- gopacket/pcap for packet capture (requires libpcap/Npcap)
- CGO_ENABLED=1 required for pcap bindings
- macOS linker warning about LC_DYSYMTAB is benign — ignore it

## Quality Checks

```bash
make ci          # lint + test-race + test-int (run before committing)
make test        # unit tests only
make test-e2e    # Docker-based E2E tests
make lint        # go vet + staticcheck
```

Pre-commit hooks enforced via Lefthook (gofmt, go vet). Pre-push runs tests.

## Releasing

### Automatic Releases (Recommended)

Every successful push to `main` automatically triggers a release:

1. **CI workflow** runs (lint, test, build verification)
2. **Release workflow** triggers automatically when CI passes
3. Version is auto-bumped (patch version increment: v0.0.1 → v0.0.2)
4. Builds binaries for all platforms
5. Creates GitHub Release with the new version tag
6. Publishes Docker images to ghcr.io
7. `notify-addon-release.yaml` fires a `repository_dispatch` to the addon repo

**IMPORTANT**: After pushing to main, ALWAYS check the GitHub Actions status:
```bash
gh run watch  # Watch CI, then release workflow
```
If any workflow fails, fix it immediately.

### Manual Releases

For specific versions (e.g., major/minor bumps), either:

**Option 1: Git tag (triggers release workflow)**
```bash
git tag v1.0.0 && git push --tags
```

**Option 2: Manual workflow dispatch**
```bash
gh workflow run release.yml -f version=v1.0.0
```

### Before Releasing

Ensure the Dockerfile version matches go.mod:
1. Check `go.mod` for Go version (e.g., `go 1.25`)
2. Update `Dockerfile` line 3: `FROM golang:1.25-alpine AS builder`
3. Update `.github/workflows/ci.yml` and `release.yml` with same version

### Release Artifacts

The release workflow produces:
1. Static Linux binaries (amd64, armv7, arm64) via Docker + QEMU
2. Native macOS (amd64, arm64) and Windows (amd64) binaries
3. GitHub Release with all artifacts
4. Docker images with tags: `latest`, `vX.Y.Z`, `X.Y.Z`, `<sha>`

See the addon repo (`jchadwick/home-assistant-addons`) for how it consumes new releases.

## Architecture Notes

- Bridge uses a two-tier context: app context (signal-only) + connection context (per peer)
- On peer disconnect, bridge returns `ErrPeerDisconnected` and main.go reconnects
- Listen mode: waits for new peer (no backoff). Connect mode: exponential backoff (1s→10s cap)
- Events are optional — NopEmitter has zero overhead when disabled
- Named FIFO at `/run/xbslink-events.pipe` bridges Go binary to bash MQTT sidecar in HA addon
- Event types: `state_changed`, `stats`, `latency`, `discovery`, `error`

## Related Repo

- HA Addon: `jchadwick/home-assistant-addons` (subdir `xbslink-ng/`)
  - s6-overlay services, bashio config, downloads prebuilt binary from GitHub releases
  - MQTT sidecar reads FIFO events and publishes to Home Assistant via MQTT discovery
