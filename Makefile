#!/usr/bin/make

# Build variables
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION ?= $(shell go version | cut -d' ' -f3)

# Build platforms
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Directories
ROOT_DIR := $(shell git rev-parse --show-toplevel)
BIN_DIR := bin
DIST_DIR := dist
BUILD_DIR := build

# Binary names with platform-specific extensions
SERVER_BINARY = wameter-server$(if $(findstring windows,$(1)),.exe)
AGENT_BINARY = wameter-agent$(if $(findstring windows,$(1)),.exe)

# Distribution archive names
DIST_NAME = wameter-$(VERSION)-$(1)-$(2)
DIST_ARCHIVE = $(DIST_DIR)/$(call DIST_NAME,$(1),$(2)).tar.gz

# Go build flags
LDFLAGS := -s -w \
	-X 'wameter/internal/version.Version=$(VERSION)' \
	-X 'wameter/internal/version.GitCommit=$(GIT_COMMIT)' \
	-X 'wameter/internal/version.BuildDate=$(BUILD_TIME)' \
	-X 'wameter/internal/version.GoVersion=$(GO_VERSION)' \
	-X 'wameter/internal/version.Platform=$(GOOS)/$(GOARCH)'

GO_BUILD_FLAGS := -trimpath -ldflags "$(LDFLAGS)"

# Test flags
TEST_FLAGS ?= -v -race -coverprofile=coverage.txt -covermode=atomic
TEST_TIMEOUT ?= 10m

.PHONY: all
all: clean verify test build

.PHONY: build
build: build-server build-agent

.PHONY: build-server
build-server:
	@echo "Building server for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)/$(GOOS)_$(GOARCH)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(GO_BUILD_FLAGS) -o $(BIN_DIR)/$(GOOS)_$(GOARCH)/$(call SERVER_BINARY,$(GOOS)) ./cmd/server

.PHONY: build-agent
build-agent:
	@echo "Building agent for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)/$(GOOS)_$(GOARCH)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(GO_BUILD_FLAGS) -o $(BIN_DIR)/$(GOOS)_$(GOARCH)/$(call AGENT_BINARY,$(GOOS)) ./cmd/agent

.PHONY: dist
dist: build
	@echo "Creating distribution package for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(DIST_DIR)
	@DIST_NAME=wameter-$(VERSION)-$(GOOS)-$(GOARCH); \
	if [ -f README.md ] && [ -f LICENSE ] && [ -d examples ]; then \
		tar -czf $(DIST_DIR)/$$DIST_NAME.tar.gz \
			-C $(BIN_DIR)/$(GOOS)_$(GOARCH) \
			$(call SERVER_BINARY,$(GOOS)) \
			$(call AGENT_BINARY,$(GOOS)) \
			-C ../../examples \
			server.example.yaml \
			agent.example.yaml \
			-C .. \
			README.md LICENSE; \
	else \
		tar -czf $(DIST_DIR)/$$DIST_NAME.tar.gz \
			-C $(BIN_DIR)/$(GOOS)_$(GOARCH) \
			$(call SERVER_BINARY,$(GOOS)) \
			$(call AGENT_BINARY,$(GOOS)); \
	fi
	@echo "Created $(DIST_DIR)/$$DIST_NAME.tar.gz"

.PHONY: build-all
build-all:
	@for platform in $(PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		echo "Building for $$GOOS/$$GOARCH..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH $(MAKE) build dist; \
	done

.PHONY: test
test:
	@echo "Running tests..."
	@go test $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-short
test-short:
	@echo "Running short tests..."
	@go test -short $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) ./...

.PHONY: test-coverage
test-coverage: test
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.txt -o coverage.html

.PHONY: bench
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

.PHONY: lint
lint:
	@echo "Running linters..."
	@golangci-lint run ./...

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w .

.PHONY: tidy
tidy:
	@echo "Tidying Go modules..."
	@go mod tidy

.PHONY: verify
verify: tidy
	@echo "Verifying module..."
	@go mod verify
	@git diff --exit-code go.mod go.sum || (echo "go.mod or go.sum is dirty" && exit 1)

.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR) $(DIST_DIR) $(BUILD_DIR)
	@rm -f coverage.txt coverage.html
	@go clean -cache -testcache -modcache

.PHONY: install
install: build
	@echo "Installing binaries..."
	@install -D -m 755 $(BIN_DIR)/$(GOOS)_$(GOARCH)/$(call SERVER_BINARY,$(GOOS)) /usr/local/bin/$(call SERVER_BINARY,$(GOOS))
	@install -D -m 755 $(BIN_DIR)/$(GOOS)_$(GOARCH)/$(call AGENT_BINARY,$(GOOS)) /usr/local/bin/$(call AGENT_BINARY,$(GOOS))

.PHONY: uninstall
uninstall:
	@echo "Uninstalling binaries..."
	@rm -f /usr/local/bin/$(call SERVER_BINARY,$(GOOS))
	@rm -f /usr/local/bin/$(call AGENT_BINARY,$(GOOS))

.PHONY: docker-build
docker-build:
	@echo "Building Docker images..."
	docker build -t wameter -f docker/server/Dockerfile .
	docker build -t wameter-agent -f docker/agent/Dockerfile .

.PHONY: docker-push
docker-push:
	@echo "Pushing Docker images..."
	docker push wameter
	docker push wameter-agent

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Clean, verify, test, and build"
	@echo "  build        - Build both server and agent binaries"
	@echo "  build-server - Build server binary only"
	@echo "  build-agent  - Build agent binary only"
	@echo "  build-all    - Build for all supported platforms"
	@echo "  dist         - Create distribution package"
	@echo "  test         - Run all tests with coverage"
	@echo "  test-short   - Run short tests"
	@echo "  bench        - Run benchmarks"
	@echo "  test-coverage- Generate coverage report"
	@echo "  lint         - Run linters"
	@echo "  fmt          - Format code"
	@echo "  tidy         - Tidy Go modules"
	@echo "  verify       - Verify dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  install      - Install binaries"
	@echo "  uninstall    - Uninstall binaries"
	@echo "  docker-build - Build Docker images"
	@echo "  docker-push  - Push Docker images"
	@echo
	@echo "Supported platforms: $(PLATFORMS)"
	@echo
	@echo "Environment variables:"
	@echo "  GOOS         - Target operating system (default: $(GOOS))"
	@echo "  GOARCH       - Target architecture (default: $(GOARCH))"
	@echo "  VERSION      - Build version (default: $(VERSION))"
	@echo "  CGO_ENABLED  - Enable CGO (default: $(CGO_ENABLED))"
