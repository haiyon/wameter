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

# Directories
BIN_DIR := bin
DIST_DIR := dist

# Binary names
SERVER_BINARY := wameter-server$(if $(filter windows,$(GOOS)),.exe)
AGENT_BINARY := wameter-agent$(if $(filter windows,$(GOOS)),.exe)

# Distribution archive name
DIST_NAME := wameter-$(GOOS)-$(GOARCH)
DIST_ARCH := $(DIST_DIR)/$(DIST_NAME).tar.gz

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

.PHONY: all
all: clean build

.PHONY: build
build: build-server build-agent

.PHONY: build-server
build-server:
	@echo "Building server for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(GO_BUILD_FLAGS) -o $(BIN_DIR)/$(SERVER_BINARY) ./cmd/server

.PHONY: build-agent
build-agent:
	@echo "Building agent for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build $(GO_BUILD_FLAGS) -o $(BIN_DIR)/$(AGENT_BINARY) ./cmd/agent

.PHONY: dist
dist: build
	@echo "Creating distribution package..."
	@mkdir -p $(DIST_DIR)
	@if [ -f README.md ] && [ -f LICENSE ] && [ -d examples ]; then \
		tar -czf $(DIST_ARCH) -C $(BIN_DIR) $(SERVER_BINARY) $(AGENT_BINARY) \
			-C .. README.md LICENSE examples/; \
	else \
		tar -czf $(DIST_ARCH) -C $(BIN_DIR) $(SERVER_BINARY) $(AGENT_BINARY); \
	fi
	@echo "Created $(DIST_ARCH)"

.PHONY: test
test:
	@echo "Running tests..."
	@go test $(TEST_FLAGS) ./...

.PHONY: test-short
test-short:
	@echo "Running short tests..."
	@go test -short $(TEST_FLAGS) ./...

.PHONY: test-coverage
test-coverage: test
	@go tool cover -html=coverage.txt -o coverage.html

.PHONY: lint
lint:
	@echo "Running linters..."
	@golangci-lint run ./...

.PHONY: verify
verify:
	@echo "Verifying module..."
	@go mod verify
	@go mod tidy
	@git diff --exit-code go.mod go.sum

.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_DIR) $(DIST_DIR)
	@rm -f coverage.txt coverage.html

.PHONY: install
install: build
	@echo "Installing binaries..."
	@install -D -m 755 $(BIN_DIR)/$(SERVER_BINARY) /usr/local/bin/$(SERVER_BINARY)
	@install -D -m 755 $(BIN_DIR)/$(AGENT_BINARY) /usr/local/bin/$(AGENT_BINARY)

.PHONY: uninstall
uninstall:
	@echo "Uninstalling binaries..."
	@rm -f /usr/local/bin/$(SERVER_BINARY)
	@rm -f /usr/local/bin/$(AGENT_BINARY)

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build both server and agent binaries"
	@echo "  build-server - Build server binary only"
	@echo "  build-agent  - Build agent binary only"
	@echo "  dist         - Create distribution package"
	@echo "  test         - Run all tests with coverage"
	@echo "  test-short   - Run short tests"
	@echo "  test-coverage- Generate coverage report"
	@echo "  lint         - Run linters"
	@echo "  verify       - Verify dependencies"
	@echo "  clean        - Clean build artifacts"
	@echo "  install      - Install binaries"
	@echo "  uninstall    - Uninstall binaries"
