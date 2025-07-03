# Go configuration
GO = go
GOFLAGS = -ldflags="-X 'github.com/larsks/airdancer/internal/version.GitHash=$(shell git rev-parse --short HEAD)' -X 'github.com/larsks/airdancer/internal/version.BuildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)'" -v

# Directories
BIN_DIR = bin

GO_SOURCES = $(shell find . -name '*.go' -not -path './bin/*')
GO_MOD_FILES = go.mod go.sum

# Main targets
BINS = $(BIN_DIR)/airdancer-api $(BIN_DIR)/airdancer-monitor $(BIN_DIR)/airdancer-ui $(BIN_DIR)/gpiotest $(BIN_DIR)/pfctl

.PHONY: all clean test test-unit test-integration help

all: $(BINS)

test: test-unit

test-unit:
	$(GO) test -v ./...

test-integration:
	./scripts/test-integration.sh

clean:
	rm -rf $(BIN_DIR)

help:
	@echo "Available targets:"
	@echo "  all              - Build all binaries"
	@echo "  clean            - Remove build artifacts"
	@echo "  test             - Run unit tests"
	@echo "  test-unit        - Run unit tests"
	@echo "  test-integration - Run integration tests"
	@echo "  install          - Install binaries to GOPATH/bin"
	@echo "  airdancer-api    - Build airdancer-api binary"
	@echo "  airdancer-monitor - Build airdancer-monitor binary"
	@echo "  airdancer-ui     - Build airdancer-ui binary"
	@echo "  gpiotest         - Build gpiotest binary"
	@echo "  pfctl            - Build pfctl binary"

# Binary targets
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(BIN_DIR)/airdancer-api: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-api

$(BIN_DIR)/airdancer-monitor: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-monitor

$(BIN_DIR)/airdancer-ui: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-ui

$(BIN_DIR)/gpiotest: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/gpiotest

$(BIN_DIR)/pfctl: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/pfctl

# Convenience targets for individual binaries
.PHONY: airdancer-api airdancer-monitor airdancer-ui gpiotest pfctl
airdancer-api: $(BIN_DIR)/airdancer-api
airdancer-monitor: $(BIN_DIR)/airdancer-monitor
airdancer-ui: $(BIN_DIR)/airdancer-ui
gpiotest: $(BIN_DIR)/gpiotest
pfctl: $(BIN_DIR)/pfctl

# Install target - build and install binaries to GOPATH/bin
.PHONY: install
install:
	$(GO) install $(GOFLAGS) ./cmd/airdancer-api
	$(GO) install $(GOFLAGS) ./cmd/airdancer-monitor
	$(GO) install $(GOFLAGS) ./cmd/airdancer-ui
	$(GO) install $(GOFLAGS) ./cmd/gpiotest
	$(GO) install $(GOFLAGS) ./cmd/pfctl 
