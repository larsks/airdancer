# Airdancer

Airdancer is a hardware automation and control system designed to run on Raspberry Pi and similar single-board computers. It provides multi-modal hardware control through web interfaces, REST APIs, email triggers, and command-line tools. The system supports PiFace Digital I/O boards and individual GPIO pins (and includes a dummy driver for resting).

## Overview

The project consists of several interconnected components:

- **API Server** (`airdancer-api`) - REST API for programmatic switch control
- **Web UI** (`airdancer-ui`) - Modern web interface for manual switch control  
- **Email Monitor** (`airdancer-monitor`) - Email-triggered automation service
- **Command Line Tools** - Direct hardware control utilities

This architecture enables remote-controlled physical displays, event-triggered automation, interactive installations, and hardware testing scenarios.

## Programs

### airdancer-api

A REST API server that provides programmatic control of switches through HTTP endpoints.

**Configuration File** (`airdancer-api.toml`):
```toml
[server]
listen = ":8080"
cors_enabled = true

[switch_collection]
driver = "dummy"

[switch_collection.dummy]
count = 8

[switch_collection.piface]
spi_device = "/dev/spidev0.0"

[switch_collection.gpio]
pins = [17, 18, 19, 20, 21, 22, 23, 24]
```

**Command Line Options**:
- `--config` - Configuration file path (default: `airdancer-api.toml`)
- `--version` - Show version information

**Example Usage**:
```bash
# Start with default configuration
airdancer-api

# Use custom configuration file
airdancer-api --config /etc/airdancer/api.toml

# Show version
airdancer-api --version
```

**API Endpoints**:
- `GET /api/switches` - List all switches and their states
- `GET /api/switches/{id}` - Get individual switch state
- `PUT /api/switches/{id}` - Set switch state
- `POST /api/switches/{id}/toggle` - Toggle switch state

### airdancer-monitor

An email monitoring service that triggers switch actions based on email patterns.

**Configuration File** (`airdancer-monitor.toml`):
```toml
[imap]
host = "imap.gmail.com"
port = 993
username = "your-email@gmail.com"
password = "your-app-password"
use_tls = true
folder = "INBOX"

[monitor]
check_interval = "30s"

[[patterns]]
name = "activation"
regex = "activate switch (\\d+)"
action = "on"

[[patterns]]
name = "deactivation"  
regex = "deactivate switch (\\d+)"
action = "off"

[switch_collection]
driver = "dummy"

[switch_collection.dummy]
count = 8
```

**Command Line Options**:
- `--config` - Configuration file path (default: `airdancer-monitor.toml`)
- `--version` - Show version information

**Example Usage**:
```bash
# Start monitoring with default configuration
airdancer-monitor

# Use custom configuration
airdancer-monitor --config /etc/airdancer/monitor.toml
```

### airdancer-ui

A web-based user interface for manual switch control.

**Configuration File** (`airdancer-ui.toml`):
```toml
[server]
listen = ":8081"
static_dir = "./internal/ui/static"

[switch_collection]
driver = "dummy"

[switch_collection.dummy]
count = 8
```

**Command Line Options**:
- `--config` - Configuration file path (default: `airdancer-ui.toml`)
- `--version` - Show version information

**Example Usage**:
```bash
# Start web UI with default configuration
airdancer-ui

# Use custom configuration
airdancer-ui --config /etc/airdancer/ui.toml

# Access web interface at http://localhost:8081
```

### pfctl

A command line tool for direct PiFace Digital I/O control.

**Command Line Options**:
- `--spi-device` - SPI device path (default: `/dev/spidev0.0`)
- `--version` - Show version information
- `--help` - Show usage information

**Subcommands**:
- `read inputs` - Read current input pin states
- `read outputs` - Read current output pin states  
- `write pin:value` - Set output pins to specified values
- `reflect` - Continuously mirror input pins to output pins

**Example Usage**:
```bash
# Read all input pins
pfctl read inputs

# Read all output pins
pfctl read outputs

# Set multiple output pins (pin:value pairs)
pfctl write 0:1 1:0 2:1

# Alternative value syntax
pfctl write 0:on 1:off 2:true 3:false

# Mirror inputs to outputs continuously
pfctl reflect

# Use alternative SPI device
pfctl --spi-device /dev/spidev0.1 read inputs

# Show version
pfctl --version
```

**Pin Values**:
- `1`, `on`, `true` - Turn pin on
- `0`, `off`, `false` - Turn pin off

### gpiotest

A simple GPIO pin testing utility for debugging hardware connections.

**Command Line Options**:
- `--version` - Show version information

**Example Usage**:
```bash
# Test GPIO functionality
gpiotest

# Show version
gpiotest --version
```

## Building

Use the provided Makefile to build all binaries:

```bash
# Build all programs
make all

# Build individual programs
make airdancer-api
make airdancer-monitor  
make airdancer-ui
make pfctl
make gpiotest

# Clean build artifacts
make clean

# Install to GOPATH/bin
make install
```

## Hardware Support

### PiFace Digital I/O

The PiFace Digital I/O board connects via SPI and provides 8 input pins and 8 output pins. Configure using:

```toml
[switch_collection]
driver = "piface"

[switch_collection.piface]
spi_device = "/dev/spidev0.0"
```

### GPIO Pins

Individual GPIO pins can be controlled using the modern character device interface:

```toml
[switch_collection]
driver = "gpio"

[switch_collection.gpio]  
pins = [17, 18, 19, 20, 21, 22, 23, 24]
```

### Dummy Driver

For testing without hardware:

```toml
[switch_collection]
driver = "dummy"

[switch_collection.dummy]
count = 8
```

## Integration Testing

The project includes comprehensive integration tests for hardware-dependent functionality. See `docs/integration-testing.md` for detailed information on running tests locally, remotely, and in CI/CD environments.

## License

This project is open source software. Please see the LICENSE file for details. 
