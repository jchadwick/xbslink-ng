# xbslink-ng Testing Guide

This document describes how to run tests for xbslink-ng.

## Quick Reference

```bash
make test           # Unit tests
make test-race      # Unit tests with race detector
make test-cover     # Unit tests with coverage report
make test-int       # Integration tests
make test-bench     # Benchmarks
make test-fuzz      # Fuzz tests (30s each)
make test-e2e       # E2E tests via Docker
make test-all       # All tests (unit + integration + bench)
```

## Test Categories

| Category    | Purpose                         | Build Tag     | Command                           |
| ----------- | ------------------------------- | ------------- | --------------------------------- |
| Unit        | Test individual functions       | (none)        | `go test ./...`                   |
| Integration | Test module interactions        | `integration` | `go test -tags=integration ./...` |
| E2E         | Test full application in Docker | -             | `make test-e2e`                   |
| Benchmark   | Performance testing             | (none)        | `go test -bench=. ./...`          |
| Fuzz        | Security/robustness             | (none)        | `go test -fuzz=... ./...`         |

## Running Tests

### Unit Tests

```bash
# Basic
go test ./...

# With race detector (recommended)
go test -race ./...

# Verbose output
go test -v ./...

# Specific package
go test -v ./internal/protocol/

# Specific test
go test -run TestParseMAC ./internal/capture/
```

### Integration Tests

Integration tests require the `integration` build tag and test actual network communication over loopback.

```bash
go test -tags=integration ./...
```

### Coverage

```bash
# Generate report
go test -coverprofile=coverage.out ./...

# View in browser
go tool cover -html=coverage.out

# Show by function
go tool cover -func=coverage.out
```

### Benchmarks

```bash
# Run all
go test -bench=. -benchmem ./...

# Specific benchmark
go test -bench=BenchmarkEncodeFrame -benchmem ./internal/protocol/

# Compare results
go test -bench=. -count=10 ./... > old.txt
# make changes
go test -bench=. -count=10 ./... > new.txt
benchstat old.txt new.txt
```

### Fuzz Tests

```bash
# Run for 30 seconds
go test -fuzz=FuzzDecode -fuzztime=30s ./internal/protocol/

# Run until failure
go test -fuzz=FuzzDecode ./internal/protocol/
```

## End-to-End Tests

E2E tests run xbslink-ng in Docker containers with simulated Xbox traffic.

### Architecture

```
Docker Network
├── bridge-a (xbslink-ng listen mode)
├── bridge-b (xbslink-ng connect mode)
└── test-runner (xbox-sim traffic generator)
```

### Running E2E Tests

```bash
# Full suite
make test-e2e

# Or manually
cd test/e2e
docker-compose up --build --abort-on-container-exit

# Cleanup
docker-compose down -v
```

### Interactive Debugging

```bash
cd test/e2e
docker-compose run --entrypoint sh bridge-a
```

## Test Structure

```
xbslink-ng/
├── internal/
│   ├── protocol/
│   │   ├── protocol_test.go       # Unit tests
│   │   ├── protocol_bench_test.go # Benchmarks
│   │   └── protocol_fuzz_test.go  # Fuzz tests
│   ├── logging/
│   │   └── logging_test.go
│   ├── capture/
│   │   ├── capture_test.go
│   │   └── capture_fuzz_test.go
│   ├── transport/
│   │   ├── transport_test.go
│   │   └── transport_integration_test.go
│   └── bridge/
│       └── bridge_test.go
└── test/
    ├── testutil/                  # Test helpers
    │   ├── helpers.go
    │   └── mocks.go
    └── e2e/                       # E2E infrastructure
        ├── Dockerfile
        ├── docker-compose.yml
        └── xbox-sim/              # Traffic simulator
```

## Test Helpers

Available in `test/testutil/`:

```go
testutil.RandomBytes(n)      // Random bytes
testutil.RandomMAC()         // Random MAC address
testutil.RandomXboxMAC()     // MAC with Xbox OUI (00:50:F2)
testutil.RandomFrame(size)   // Valid Ethernet frame
testutil.FreePort()          // Available UDP port
testutil.WaitFor(timeout, condition) // Poll until true
```

## CI Integration

The Makefile `ci` target mirrors GitHub Actions:

```bash
make ci
# Runs: lint -> test-race -> test-int
```

## Troubleshooting

| Issue                              | Solution                                 |
| ---------------------------------- | ---------------------------------------- |
| Permission denied on capture tests | Run with `sudo` or set pcap capabilities |
| Npcap not installed (Windows)      | Install Npcap with WinPcap compatibility |
| E2E tests timeout                  | Increase Docker resources                |
| Port conflicts                     | Ensure ports 31415+ are free             |
