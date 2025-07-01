BIN_DIR = bin
GO = go
GOFLAGS = -ldflags="-s -w"

BINS = $(BIN_DIR)/airdancer-api $(BIN_DIR)/airdancer-monitor $(BIN_DIR)/airdancer-ui $(BIN_DIR)/gpiotest $(BIN_DIR)/piface-reflector

# Find all Go source files using go list (more accurate than find)
GO_SOURCES = $(shell go list -f '{{$$dir := .Dir}}{{range .GoFiles}}{{$$dir}}/{{.}} {{end}}' ./...)
GO_MOD_FILES = go.mod go.sum

.PHONY: all
all: $(BINS)

# Create bin directory
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# Individual binary targets - these are real file targets that depend on source files
$(BIN_DIR)/airdancer-api: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-api

$(BIN_DIR)/airdancer-monitor: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-monitor

$(BIN_DIR)/airdancer-ui: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/airdancer-ui

$(BIN_DIR)/gpiotest: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/gpiotest

$(BIN_DIR)/piface-reflector: $(GO_SOURCES) $(GO_MOD_FILES) | $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $@ ./cmd/piface-reflector

# Convenience targets for individual binaries
.PHONY: airdancer-api airdancer-monitor airdancer-ui gpiotest piface-reflector
airdancer-api: $(BIN_DIR)/airdancer-api
airdancer-monitor: $(BIN_DIR)/airdancer-monitor
airdancer-ui: $(BIN_DIR)/airdancer-ui
gpiotest: $(BIN_DIR)/gpiotest
piface-reflector: $(BIN_DIR)/piface-reflector

# Clean target - remove built binaries
.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

# Install target - build and install binaries to GOPATH/bin
.PHONY: install
install:
	$(GO) install $(GOFLAGS) ./cmd/airdancer-api
	$(GO) install $(GOFLAGS) ./cmd/airdancer-monitor
	$(GO) install $(GOFLAGS) ./cmd/airdancer-ui
	$(GO) install $(GOFLAGS) ./cmd/gpiotest
	$(GO) install $(GOFLAGS) ./cmd/piface-reflector

# Test target - run all tests
.PHONY: test
test:
	$(GO) test ./...

# Build with race detection (useful for development)
.PHONY: race
race: GOFLAGS := -race
race: all

# Build with debug symbols (useful for debugging)
.PHONY: debug
debug: GOFLAGS := -gcflags="all=-N -l"
debug: all

# Force rebuild of all binaries
.PHONY: rebuild
rebuild: clean all

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all              - Build all binaries (default)"
	@echo "  airdancer-api    - Build airdancer-api binary"
	@echo "  airdancer-monitor - Build airdancer-monitor binary"
	@echo "  airdancer-ui     - Build airdancer-ui binary"
	@echo "  gpiotest         - Build gpiotest binary"
	@echo "  piface-reflector - Build piface-reflector binary"
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
