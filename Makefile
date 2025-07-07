# Go configuration
PKG = github.com/larsks/airdancer
GO = go
GOLDFLAGS = \
			-s \
			-w \
	    -X '$(PKG)/internal/version.BuildVersion=$(VERSION)' \
	    -X '$(PKG)/internal/version.BuildRef=$(COMMIT)' \
	    -X '$(PKG)/internal/version.BuildDate=$(DATE)'
GOFLAGS = -ldflags="$(GOLDFLAGS)"

# Directories
BIN_DIR = bin

# Find all Go source files using go list (more accurate than find)
GO_SOURCES = $(shell go list -f '{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}' ./...)
GO_MOD_FILES = go.mod go.sum

VERSION = $(shell git describe --tags --exact-match 2> /dev/null || echo dev)
COMMIT = $(shell git rev-parse --short=10 HEAD)
DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%S")

# Main targets
BINS = $(BIN_DIR)/airdancer-api \
	$(BIN_DIR)/airdancer-monitor \
	$(BIN_DIR)/airdancer-ui \
	$(BIN_DIR)/airdancer-buttons \
	$(BIN_DIR)/gpiotest \
	$(BIN_DIR)/pfctl \
	$(BIN_DIR)/configvalidate

.PHONY: all help

all: $(BINS)

# Test target - run all tests
.PHONY: test
test: test-unit

# Unit tests
.PHONY: test-unit
test-unit:
	$(GO) test -v ./...

# Integration tests
.PHONY: test-integration
test-integration:
	./scripts/test-integration.sh

# Clean target - remove built binaries
.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

# Build with race detection (useful for development)
.PHONY: race
race: GOFLAGS := -race
race: all

# Build with debug symbols (useful for debugging)
.PHONY: debug
debug: GOFLAGS := -gcflags="all=-N -l"
debug: all

# Rebuild target - clean and build all
.PHONY: rebuild
rebuild: clean all

# Help target - show available targets
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all              - Build all binaries (default)"
	@echo "  airdancer-api    - Build airdancer-api binary"
	@echo "  airdancer-monitor - Build airdancer-monitor binary"
	@echo "  airdancer-ui     - Build airdancer-ui binary"
	@echo "  gpiotest         - Build gpiotest binary"
	@echo "  pfctl            - Build pfctl binary"
	@echo "  configvalidate   - Build configvalidate binary"
	@echo "  clean            - Remove built binaries"
	@echo "  install          - Install binaries to GOPATH/bin"
	@echo "  test             - Run all tests"
	@echo "  race             - Build with race detection"
	@echo "  debug            - Build with debug symbols"
	@echo "  rebuild          - Force rebuild of all binaries"
	@echo "  help             - Show this help message"
	@echo ""
	@echo "Dependencies:"
	@echo "  Binaries will only rebuild when Go source files or go.mod/go.sum change"
	@echo "  Uses 'go list' to accurately track all Go files (including test files)"

# Binary targets
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(BIN_DIR)/airdancer-api: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-api

$(BIN_DIR)/airdancer-buttons: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-buttons

$(BIN_DIR)/airdancer-monitor: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-monitor

$(BIN_DIR)/airdancer-ui: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-ui

$(BIN_DIR)/gpiotest: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/gpiotest

$(BIN_DIR)/pfctl: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/pfctl

$(BIN_DIR)/configvalidate: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/configvalidate

# Convenience targets for individual binaries
.PHONY: airdancer-api airdancer-monitor airdancer-ui gpiotest pfctl configvalidate
airdancer-api: $(BIN_DIR)/airdancer-api
airdancer-monitor: $(BIN_DIR)/airdancer-monitor
airdancer-ui: $(BIN_DIR)/airdancer-ui
gpiotest: $(BIN_DIR)/gpiotest
pfctl: $(BIN_DIR)/pfctl
configvalidate: $(BIN_DIR)/configvalidate

# Install target - build and install binaries to GOPATH/bin
.PHONY: install
install:
	$(GO) install $(GOFLAGS) ./cmd/airdancer-api
	$(GO) install $(GOFLAGS) ./cmd/airdancer-monitor
	$(GO) install $(GOFLAGS) ./cmd/airdancer-ui
	$(GO) install $(GOFLAGS) ./cmd/gpiotest
	$(GO) install $(GOFLAGS) ./cmd/pfctl
	$(GO) install $(GOFLAGS) ./cmd/configvalidate 
