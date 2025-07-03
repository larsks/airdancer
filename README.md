# Airdancer

Airdancer is a hardware automation and control system designed to run on Raspberry Pi and similar single-board computers. It provides multi-modal hardware control through web interfaces, REST APIs, email triggers, and command-line tools. The system supports PiFace Digital I/O boards and individual GPIO pins (and includes a dummy driver for resting).

## Overview

The project consists of several interconnected components:

- **API Server** (`airdancer-api`) - REST API for programmatic switch control
- **Web UI** (`airdancer-ui`) - Modern web interface for manual switch control  
- **Email Monitor** (`airdancer-monitor`) - Email-triggered automation service
- **Command Line Tools** - Direct hardware control utilities

This architecture enables remote-controlled physical displays, event-triggered automation, interactive installations, and IoT automation projects.

## Hardware Support

- **PiFace Digital I/O** - 8-pin digital I/O expansion board via SPI
- **GPIO Pins** - Direct Raspberry Pi GPIO control using modern character devices
- **Dummy Driver** - Software simulation for development and testing

## Programs

### airdancer-api

A REST API server that provides programmatic control of hardware switches through multiple drivers.

#### Description

The API server exposes HTTP endpoints for controlling switches, reading states, and managing timed operations. It supports PiFace Digital I/O boards, GPIO pins, and a dummy driver for testing. The server includes automatic switch initialization, timed operations with auto-off functionality, and comprehensive error handling.

#### Configuration File

Configuration is loaded from TOML files with the following structure:

```toml
listen-address = "127.0.0.1"
listen-port = 8080
driver = "piface"

[piface]
spidev = "/dev/spidev0.0"

[gpio]
pins = ["GPIO18", "GPIO19", "GPIO20", "GPIO21"]

[dummy]
switch_count = 4
```

#### Command Line Options

```
--config string              Config file to use
--listen-address string      Listen address for http server (default "")
--listen-port int            Listen port for http server (default 8080)
--driver string              Driver to use (piface, gpio, or dummy) (default "dummy")
--piface.spidev string       SPI device to use (default "/dev/spidev0.0")
--gpio.pins strings          GPIO pins to use (for gpio driver)
--dummy.switch-count uint    Number of switches for dummy driver (default 4)
--version                    Show version and exit
```

#### Example Usage

```bash
# Start with dummy driver for testing
airdancer-api --driver dummy --dummy.switch-count 8

# Start with PiFace driver
airdancer-api --driver piface --piface.spidev /dev/spidev0.0

# Start with GPIO driver
airdancer-api --driver gpio --gpio.pins GPIO18,GPIO19,GPIO20

# Use configuration file
airdancer-api --config /etc/airdancer/api.toml

# Test API endpoints
curl http://localhost:8080/switch/all
curl -X POST -H "Content-Type: application/json" -d '{"state":"on"}' http://localhost:8080/switch/0
curl -X POST -H "Content-Type: application/json" -d '{"state":"on","duration":30}' http://localhost:8080/switch/all
```

### airdancer-monitor

An email monitoring service that watches IMAP mailboxes for messages matching regex patterns and executes commands when matches are found.

#### Description

The monitor connects to IMAP servers, continuously checks for new emails, and executes configured commands when email content matches specified regex patterns. It supports both SSL and non-SSL connections, provides environment variables with email metadata to executed commands, and includes comprehensive error handling with automatic reconnection.

#### Configuration File

```toml
[imap]
server = "imap.gmail.com"
port = 993
username = "your-email@gmail.com"
password = "your-app-password"
use_ssl = true
mailbox = "INBOX"

[monitor]
regex_pattern = "urgent.*alert|emergency.*notification"
command = "curl -X POST -H 'Content-Type: application/json' -d '{\"state\":\"on\"}' http://localhost:8080/switch/all"
check_interval_seconds = 30
```

#### Command Line Options

```
--config string                      Config file to use
--imap.server string                 IMAP server address
--imap.port int                      IMAP server port (default 993)
--imap.username string               IMAP username
--imap.password string               IMAP password
--imap.use-ssl                       Use SSL for IMAP connection (default true)
--imap.mailbox string                IMAP mailbox to monitor (default "INBOX")
--monitor.regex-pattern string       Regex pattern to match in email bodies
--monitor.command string             Command to execute on regex match
--monitor.check-interval int         Interval in seconds to check for new emails (default 30)
--version                            Show version and exit
```

#### Example Usage

```bash
# Start with configuration file
airdancer-monitor --config /etc/airdancer/monitor.toml

# Start with command line options
airdancer-monitor \
  --imap.server imap.gmail.com \
  --imap.username alerts@company.com \
  --imap.password app-password \
  --monitor.regex-pattern "CRITICAL.*ERROR" \
  --monitor.command "airdancer-api-call.sh"

# Monitor different mailbox
airdancer-monitor \
  --config /etc/airdancer/monitor.toml \
  --imap.mailbox "Alerts"
```

#### Environment Variables

When a command is executed, the following environment variables are available:

- `EMAIL_FROM` - Sender's email address
- `EMAIL_SUBJECT` - Email subject line
- `EMAIL_DATE` - Email date in RFC3339 format
- `EMAIL_UID` - Email UID from IMAP server

The email body is provided via stdin to the executed command.

### airdancer-ui

A web-based user interface for controlling switches through a modern, responsive web interface.

#### Description

The UI server provides a beautiful, real-time web interface for switch control. It connects to the airdancer-api service and offers individual switch control, master control for all switches, real-time status updates, and a responsive design that works on desktop and mobile devices.

#### Configuration File

```toml
listen-address = "0.0.0.0"
listen-port = 8081
api-base-url = "http://localhost:8080"
```

#### Command Line Options

```
--config string              Config file to use
--listen-address string      Listen address for UI server (default "")
--listen-port int            Listen port for UI server (default 8081)
--api-base-url string        Base URL for the API server (default "http://localhost:8080")
--version                    Show version and exit
```

#### Example Usage

```bash
# Start UI server with defaults
airdancer-ui

# Start on all interfaces
airdancer-ui --listen-address 0.0.0.0

# Connect to remote API server
airdancer-ui --api-base-url http://192.168.1.100:8080

# Use configuration file
airdancer-ui --config /etc/airdancer/ui.toml
```

Access the web interface at `http://localhost:8081` (or your configured address).

### pfctl

A command-line tool for direct control of PiFace Digital I/O boards.

#### Description

pfctl provides low-level access to PiFace Digital I/O boards for reading input states, reading output states, and setting individual output pins. It's useful for debugging, scripting, and direct hardware control.

#### Command Line Options

```
--spi-device string     SPI device path (default "/dev/spidev0.0")
--version               Show version and exit
-h, --help             Show help
```

#### Commands

- `read inputs` - Read current input pin states
- `read outputs` - Read current output pin states  
- `write pin:value` - Set output pins to specified values

#### Pin Values

- `on`, `1`, `true` - Turn pin on
- `off`, `0`, `false` - Turn pin off

#### Example Usage

```bash
# Read all input pins
pfctl read inputs

# Read all output pins
pfctl read outputs

# Set individual pins
pfctl write 0:on 1:off 2:on

# Set multiple pins with different syntax
pfctl write 0:1 1:0 2:true 3:false

# Use different SPI device
pfctl --spi-device /dev/spidev0.1 read inputs

# Turn on all outputs
pfctl write 0:on 1:on 2:on 3:on 4:on 5:on 6:on 7:on
```

### gpiotest

A command-line tool for testing and controlling GPIO pins.

#### Description

gpiotest provides direct GPIO pin control for testing and automation. It supports both ActiveHigh and ActiveLow polarities and can control multiple pins simultaneously.

#### Command Line Options

```
--polarity string    GPIO polarity: ActiveHigh or ActiveLow (default "ActiveHigh")
```

#### Example Usage

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

### piface-reflector

A utility that continuously mirrors PiFace input states to output pins.

#### Description

piface-reflector reads the state of PiFace input pins and immediately reflects them to the corresponding output pins. This creates a real-time mirror where output pin N matches the state of input pin N. It's useful for testing, debugging, and creating input-driven displays.

#### Features

- Real-time input-to-output mirroring
- 10ms polling interval for responsive updates
- Displays input changes on console
- Runs continuously until interrupted

#### Example Usage

```bash
# Start mirroring with default SPI device (/dev/spidev0.0)
piface-reflector

# Monitor the console output to see input changes
# Input pins 0-7 will be mirrored to output pins 0-7
# Press Ctrl+C to stop
```

## Building

### Prerequisites

- Go 1.19 or later
- Hardware access (for non-dummy drivers):
  - Raspberry Pi or compatible board
  - PiFace Digital I/O board (for PiFace driver)
  - GPIO access permissions

### Build All Programs

```bash
# Build all binaries
make all

# Build individual programs
make airdancer-api
make airdancer-monitor
make airdancer-ui
make pfctl
make gpiotest
make piface-reflector

# Install to GOPATH/bin
make install

# Clean build artifacts
make clean
```

### Cross-compilation

```bash
# Build for Raspberry Pi (ARM64)
GOOS=linux GOARCH=arm64 go build ./cmd/airdancer-api

# Build for Raspberry Pi (ARM32)
GOOS=linux GOARCH=arm go build ./cmd/airdancer-api
```

## Hardware Setup

### PiFace Digital I/O

1. Enable SPI interface:
   ```bash
   sudo raspi-config
   # Interface Options -> SPI -> Enable
   sudo reboot
   ```

2. Verify SPI device:
   ```bash
   ls -l /dev/spidev*
   ```

### GPIO Access

1. Add user to gpio group:
   ```bash
   sudo usermod -a -G gpio $USER
   # Log out and back in
   ```

2. Verify GPIO access:
   ```bash
   ls -l /dev/gpiochip*
   ```

## System Integration

### Systemd Services

Create service files for automatic startup:

```ini
# /etc/systemd/system/airdancer-api.service
[Unit]
Description=Airdancer API Server
After=network.target

[Service]
Type=simple
User=pi
ExecStart=/usr/local/bin/airdancer-api --config /etc/airdancer/api.toml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start services
sudo systemctl enable airdancer-api
sudo systemctl start airdancer-api
sudo systemctl status airdancer-api
```

### Complete System Example

```bash
# 1. Start API server with PiFace driver
airdancer-api --driver piface &

# 2. Start UI server
airdancer-ui &

# 3. Start email monitor
airdancer-monitor --config monitor.toml &

# 4. Test with command line tools
pfctl read inputs
gpiotest GPIO23:on

# 5. Access web interface
open http://localhost:8081
```

## Testing

```bash
# Run unit tests
make test

# Run integration tests (requires hardware)
./scripts/test-integration.sh

# Run tests on remote hardware
./scripts/test-remote.sh pi@raspberrypi.local
```

## License

[Add your license information here]

## Contributing

[Add contribution guidelines here] 
