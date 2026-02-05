# xbslink-ng Makefile

.PHONY: all build test test-race test-cover test-int test-bench test-fuzz test-e2e test-all clean

# Default target
all: build

# Build the main binary
build:
	go build -ldflags="-s -w" -o xbslink-ng ./cmd/xbslink-ng

# Run unit tests
test:
	go test -v ./...

# Run unit tests with race detector
test-race:
	go test -v -race ./...

# Run unit tests with coverage
test-cover:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run integration tests
test-int:
	go test -v -tags=integration ./...

# Run benchmarks
test-bench:
	go test -bench=. -benchmem ./...

# Run fuzz tests (30 seconds each)
test-fuzz:
	@echo "Running fuzz tests (30s each)..."
	go test -fuzz=FuzzDecode -fuzztime=30s ./internal/protocol/ || true
	go test -fuzz=FuzzParseMAC -fuzztime=30s ./internal/capture/ || true

# Run E2E tests via Docker
test-e2e:
	@echo "Building E2E test infrastructure..."
	docker-compose -f test/e2e/docker-compose.yml build
	@echo "Running E2E tests..."
	docker-compose -f test/e2e/docker-compose.yml up --abort-on-container-exit --exit-code-from test-runner
	@echo "Cleaning up..."
	docker-compose -f test/e2e/docker-compose.yml down -v

# Run E2E tests (build only, for debugging)
test-e2e-build:
	docker-compose -f test/e2e/docker-compose.yml build

# Clean up E2E resources
test-e2e-clean:
	docker-compose -f test/e2e/docker-compose.yml down -v --rmi local

# Run all tests
test-all: test-race test-int test-bench

# Lint
lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

# Clean build artifacts
clean:
	rm -f xbslink-ng coverage.out coverage.html

# CI target (mimics GitHub Actions)
ci: lint test-race test-int
	@echo "CI checks passed!"

# Help
help:
	@echo "Available targets:"
	@echo "  build       - Build the xbslink-ng binary"
	@echo "  test        - Run unit tests"
	@echo "  test-race   - Run unit tests with race detector"
	@echo "  test-cover  - Run unit tests with coverage report"
	@echo "  test-int    - Run integration tests"
	@echo "  test-bench  - Run benchmarks"
	@echo "  test-fuzz   - Run fuzz tests (30s each)"
	@echo "  test-e2e    - Run E2E tests via Docker"
	@echo "  test-all    - Run all tests (unit + integration + bench)"
	@echo "  lint        - Run linters"
	@echo "  ci          - Run CI checks"
	@echo "  clean       - Clean build artifacts"
