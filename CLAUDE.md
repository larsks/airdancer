# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Building
```bash
# Build all binaries
make all

# Build specific programs
make airdancer-api
make airdancer-monitor
make airdancer-ui
make airdancer-buttons
make pfctl
make gpiotest
make configvalidate

# Build variants
make race       # Build with race detection
make debug      # Build with debug symbols
make rebuild    # Clean and rebuild
```

### Testing
```bash
# Run unit tests
make test
# OR
go test ./...

# Run integration tests (requires hardware)
./scripts/test-integration.sh all        # All integration tests
./scripts/test-integration.sh piface     # PiFace-specific tests
./scripts/test-integration.sh gpio       # GPIO-specific tests
./scripts/test-integration.sh -v all     # Verbose output
./scripts/test-integration.sh -r all     # With race detector

# Run integration tests on remote hardware
./scripts/test-remote.sh pi@192.168.1.100
./scripts/test-remote.sh -d piface -u pi rpi.local
```

### Code Quality
```bash
# Format code (always run after changes)
go fmt ./...

# Validate configuration files
./bin/configvalidate --type api --config examples/airdancer-api.toml
./bin/configvalidate --type ui --config examples/airdancer-ui.toml
./bin/configvalidate --type monitor --config examples/airdancer-monitor.toml
```

### Running Services
```bash
# Start API server (default: localhost:8080)
./bin/airdancer-api --config examples/airdancer-api.toml

# Start web UI (default: localhost:8081)
./bin/airdancer-ui --config examples/airdancer-ui.toml

# Start email monitor
./bin/airdancer-monitor --config examples/airdancer-monitor.toml

# Start button monitoring
./bin/airdancer-buttons --config examples/airdancer-buttons.toml
```

## Architecture Overview

### Core Components

**Airdancer** is a modular IoT control system with a microservices architecture:

- **airdancer-api**: REST API server for remote switch control
- **airdancer-ui**: Web interface for switch management  
- **airdancer-buttons**: Button monitoring and action execution
- **airdancer-monitor**: Email monitoring with regex pattern matching
- **airdancer-powerbutton**: Specialized power button handling

### Hardware Abstraction

The system uses a **driver pattern** for hardware abstraction through the `SwitchCollection` interface:

- **Dummy Driver**: Virtual switches for testing (`internal/switchcollection/dummy/`)
- **GPIO Driver**: Direct GPIO pin control via periph.io (`internal/switchcollection/gpio/`)
- **PiFace Driver**: SPI-based PiFace Digital board control (`internal/piface/`)

### Key Architectural Patterns

1. **Event-Driven Architecture**: Components communicate through typed channels
2. **Dependency Injection**: Services accept interfaces for testability
3. **Configuration-as-Code**: Consistent TOML/flag/env var configuration
4. **Driver Pattern**: Pluggable hardware drivers with common interfaces

### Directory Structure

```
cmd/                    # Main binaries
internal/
├── api/               # REST API server implementation
├── buttondriver/      # Button input drivers (GPIO, event)
├── buttonwatcher/     # Button monitoring service
├── monitor/           # Email monitoring service
├── switchcollection/  # Hardware switch abstraction
├── piface/           # PiFace Digital board driver
├── ui/               # Web interface server
├── config/           # Configuration loading utilities
└── events/           # Event type definitions
```

### Configuration Management

All services use a unified configuration pattern:
- **TOML files**: Primary configuration format
- **Command-line flags**: Override config file values
- **Environment variables**: Secondary override mechanism
- **Validation**: Strict validation with `configvalidate` tool

### Testing Strategy

- **Unit Tests**: `go test ./...` - Run on any machine
- **Integration Tests**: Require actual hardware (Raspberry Pi)
  - Use build tags: `//go:build integration && piface`
  - Test scripts in `scripts/` directory
  - Remote testing support via SSH

## Hardware Requirements

### For Development
- Any machine with Go 1.23+ for unit tests
- Raspberry Pi for integration tests

### For Production
- Raspberry Pi or compatible SBC
- Optional: PiFace Digital I/O board
- GPIO access (user in `gpio` group or root)

## Service Communication

- **API Server**: HTTP REST endpoints on port 8080
- **Web UI**: HTTP server on port 8081, calls API server
- **Email Monitor**: IMAP client, executes shell commands
- **Button Service**: Reads input devices, executes actions

## Development Guidelines

- Always run `go fmt` after making changes
- Always run tests after making changes
- Update example files in `examples/` directory when making changes that effect configuration
- Prefer `embed` package over literal text in tests
- Use `for i := range LIMIT` style loops
- Validate generated config files with `configvalidate`

## Error Handling

The codebase implements structured error handling:
- Service-specific error types in dedicated `errors.go` files
- Context-preserving error wrapping
- Graceful degradation where possible
- Proper resource cleanup with defer statements
