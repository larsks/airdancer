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
# HTTP server configuration
listen-address = ""  # Leave empty to listen on all interfaces
listen-port = 8080

# Switch driver configuration
driver = "dummy"  # Options: "dummy", "piface", "gpio"

# Dummy driver configuration (for testing without hardware)
[dummy]
switch_count = 8

# PiFace driver configuration (for PiFace Digital I/O boards)
[piface]
spidev = "/dev/spidev0.0"

# GPIO driver configuration (for individual GPIO pins)
[gpio]
pins = ["GPIO17", "GPIO18", "GPIO19", "GPIO20", "GPIO21", "GPIO22", "GPIO23", "GPIO24"]
```

**Command Line Options**:
- `--config string` - Configuration file to use
- `--driver string` - Driver to use (piface, gpio, or dummy) (default: "dummy")
- `--dummy.switch-count uint` - Number of switches for dummy driver (default: 4)
- `--gpio.pins strings` - GPIO pins to use (for gpio driver)
- `--listen-address string` - Listen address for HTTP server (default: all interfaces)
- `--listen-port int` - Listen port for HTTP server (default: 8080)
- `--piface.spidev string` - SPI device to use (default: "/dev/spidev0.0")
- `--version` - Show version and exit

**Example Usage**:
```bash
# Start with default configuration
airdancer-api

# Use custom configuration file
airdancer-api --config /etc/airdancer/api.toml

# Start with dummy driver for testing
airdancer-api --driver dummy --dummy.switch-count 8

# Start with PiFace driver
airdancer-api --driver piface --piface.spidev /dev/spidev0.0

# Start with GPIO driver
airdancer-api --driver gpio --gpio.pins GPIO18,GPIO19,GPIO20

# Start on specific address and port
airdancer-api --listen-address 127.0.0.1 --listen-port 9000

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
# IMAP server configuration
[imap]
server = "imap.gmail.com"
port = 993
username = "your-email@gmail.com"
password = "your-app-password"
use_ssl = true
mailbox = "INBOX"

# Monitor configuration
[monitor]
regex_pattern = "activate switch (\\d+)"
command = "echo 'Switch activation request: $1'"
check_interval_seconds = 30
```

**Command Line Options**:
- `--config string` - Configuration file to use
- `--imap.mailbox string` - IMAP mailbox to monitor (default: "INBOX")
- `--imap.password string` - IMAP password
- `--imap.port int` - IMAP server port (default: 993)
- `--imap.server string` - IMAP server address
- `--imap.use-ssl` - Use SSL for IMAP connection (default: true)
- `--imap.username string` - IMAP username
- `--monitor.check-interval int` - Interval in seconds to check for new emails (default: 30)
- `--monitor.command string` - Command to execute on regex match
- `--monitor.regex-pattern string` - Regex pattern to match in email bodies
- `--version` - Show version and exit

**Example Usage**:
```bash
# Start monitoring with default configuration
airdancer-monitor

# Use custom configuration
airdancer-monitor --config /etc/airdancer/monitor.toml

# Start with command line options
airdancer-monitor \
  --imap.server imap.gmail.com \
  --imap.username alerts@company.com \
  --imap.password app-password \
  --monitor.regex-pattern "CRITICAL.*ERROR" \
  --monitor.command "echo 'Alert received'"

# Monitor different mailbox
airdancer-monitor \
  --config /etc/airdancer/monitor.toml \
  --imap.mailbox "Alerts"
```

**Environment Variables**:

When a command is executed, the following environment variables are available:

- `EMAIL_FROM` - Sender's email address
- `EMAIL_SUBJECT` - Email subject line
- `EMAIL_DATE` - Email date in RFC3339 format
- `EMAIL_UID` - Email UID from IMAP server

The email body is provided via stdin to the executed command.

### airdancer-ui

A web-based user interface for manual switch control.

**Configuration File** (`airdancer-ui.toml`):
```toml
# HTTP server configuration
listen-address = ""  # Leave empty to listen on all interfaces
listen-port = 8081

# API server configuration
api-base-url = "http://localhost:8080"
```

**Command Line Options**:
- `--api-base-url string` - Base URL for the API server (default: "http://localhost:8080")
- `--config string` - Configuration file to use
- `--listen-address string` - Listen address for UI server (default: all interfaces)
- `--listen-port int` - Listen port for UI server (default: 8081)
- `--version` - Show version and exit

**Example Usage**:
```bash
# Start web UI with default configuration
airdancer-ui

# Use custom configuration
airdancer-ui --config /etc/airdancer/ui.toml

# Start on all interfaces
airdancer-ui --listen-address 0.0.0.0

# Connect to remote API server
airdancer-ui --api-base-url http://192.168.1.100:8080

# Start on specific port
airdancer-ui --listen-port 9080

# Access web interface at http://localhost:8081
```

### pfctl

A command line tool for direct PiFace Digital I/O control.

**Command Line Options**:
- `--spi-device string` - SPI device path (default: "/dev/spidev0.0")
- `--version` - Show version and exit
- `-h, --help` - Show help

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

A command-line tool for testing and controlling GPIO pins.

**Command Line Options**:
- `--polarity string` - GPIO polarity: ActiveHigh or ActiveLow (default: "ActiveHigh")

**Usage Pattern**:
```
gpiotest [--polarity ActiveHigh|ActiveLow] gpio_name:value [gpio_name:value...]
```

**Example Usage**:
```bash
# Control GPIO pins with default ActiveHigh polarity
gpiotest GPIO23:on GPIO24:off

# Use ActiveLow polarity
gpiotest --polarity ActiveLow GPIO23:on GPIO24:off

# Control multiple pins
gpiotest GPIO18:on GPIO19:off GPIO20:on GPIO21:off

# Different value formats
gpiotest GPIO23:1 GPIO24:0
gpiotest GPIO23:true GPIO24:false
```

**Pin Values**:
- `1`, `on`, `true` - Turn pin on
- `0`, `off`, `false` - Turn pin off

### configvalidate

A configuration validation tool that validates configuration files for the airdancer programs.

**Command Line Options**:
- `--config string` - Configuration file to validate
- `--type string` - Configuration type: api, ui, or monitor
- `-h, --help` - Show help
- `--version` - Show version and exit

**Example Usage**:
```bash
# Validate API configuration
configvalidate --type api --config airdancer-api.toml

# Validate UI configuration
configvalidate --type ui --config airdancer-ui.toml

# Validate monitor configuration
configvalidate --type monitor --config airdancer-monitor.toml

# Show help
configvalidate --help

# Show version
configvalidate --version
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
make configvalidate

# Clean build artifacts
make clean

# Install to GOPATH/bin
make install
```

## Hardware Support

### PiFace Digital I/O

The PiFace Digital I/O board connects via SPI and provides 8 input pins and 8 output pins. Configure using:

```toml
driver = "piface"

[piface]
spidev = "/dev/spidev0.0"
```

### GPIO Pins

Individual GPIO pins can be controlled using the modern character device interface:

```toml
driver = "gpio"

[gpio]
pins = ["GPIO17", "GPIO18", "GPIO19", "GPIO20", "GPIO21", "GPIO22", "GPIO23", "GPIO24"]
```

### Dummy Driver

For testing without hardware:

```toml
driver = "dummy"

[dummy]
switch_count = 8
```

## Integration Testing

The project includes comprehensive integration tests for hardware-dependent functionality. See `docs/integration-testing.md` for detailed information on running tests locally, remotely, and in CI/CD environments.

## License

This project is open source software. Please see the LICENSE file for details. 
