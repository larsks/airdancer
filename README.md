# Airdancer

Airdancer is a hardware automation and control system designed to run on Raspberry Pi and similar single-board computers. It provides multi-modal hardware control through web interfaces, REST APIs, email triggers, and command-line tools. The system supports switches managed by PiFace Digital I/O boards and by individual GPIO pins. A dummy driver is available for testing without specialized hardware.

## Overview

The project consists of the following interconnected components:

- `airdancer-api` - REST API for programmatic switch control
- `airdancer-ui` - Web interface for manual switch control  
- `airdancer-monitor` - Email-triggered automation service
- `airdancer-wifi-fallback` - Automatic WiFi hotspot fallback service

There are also a number of command line tools for testing and diagnostic purposes.

## Programs

### airdancer-api

A REST API server that provides programmatic control of switches through HTTP endpoints.

#### Configuration file

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

#### Command line options

- `--config string` - Configuration file to use
- `--driver string` - Driver to use (piface, gpio, or dummy) (default: "dummy")
- `--dummy.switch-count uint` - Number of switches for dummy driver (default: 4)
- `--gpio.pins strings` - GPIO pins to use (for gpio driver)
- `--listen-address string` - Listen address for HTTP server (default: all interfaces)
- `--listen-port int` - Listen port for HTTP server (default: 8080)
- `--piface.spidev string` - SPI device to use (default: "/dev/spidev0.0")
- `--version` - Show version and exit

#### Example usage

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

#### API endpoints

- `GET /api/switch/all` - List all switches and their states
- `POST /api/switch/all` - Control all switches at the same time
- `GET /api/switches/{id}` - Get individual switch state
- `POST /api/switches/{id}` - Control individual switch state

### airdancer-monitor

An email monitoring service that triggers switch actions based on email patterns.

#### Configuration file

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
command = "http localhost:8080 state=on duration:=10"
check_interval_seconds = 30
```

#### Command Line Options

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

#### Example Usage

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

#### Environment Variables

When the monitor executes a command, the following environment variables are available:

- `EMAIL_FROM` - Sender's email address
- `EMAIL_SUBJECT` - Email subject line
- `EMAIL_DATE` - Email date in RFC3339 format
- `EMAIL_UID` - Email UID from IMAP server

The email body is available on stdin of the executed command.

### airdancer-wifi-fallback

A WiFi hotspot fallback service that automatically enables hotspot mode when NetworkManager cannot establish a connection to known networks. This is essential for Raspberry Pi devices that may start up in a location for which they don't have pre-configured WiFi credentials.

#### Configuration file

```bash
# WiFi interface to use for both connectivity and hotspot mode
AIRDANCER_WIFI_INTERFACE=wlan0

# Hotspot network settings
AIRDANCER_HOTSPOT_SSID=AirdancerSetup
AIRDANCER_HOTSPOT_PASSWORD=airdancer123

# Monitoring configuration (in seconds)
AIRDANCER_CONNECTION_TIMEOUT=120       # How long to wait for NetworkManager to establish connection
AIRDANCER_CHECK_INTERVAL=5            # How often to check connection status during timeout period

# Logging level: DEBUG, INFO, WARN, ERROR
AIRDANCER_LOG_LEVEL=INFO
```

#### Command Line Options

- `-c, --config FILE` - Configuration file (default: `/etc/airdancer/wifi-fallback.conf`)
- `-i, --interface IFACE` - WiFi interface (default: `wlan0`)
- `-s, --ssid SSID` - Hotspot SSID (default: `AirdancerSetup`)
- `-p, --password PASS` - Hotspot password (default: `airdancer123`)
- `-t, --timeout SECONDS` - Connection timeout (default: `120`)
- `-v, --verbose` - Enable debug logging
- `--help` - Show help message

#### Example Usage

```bash
# Run with default settings
airdancer-wifi-fallback.sh

# Run with custom interface
airdancer-wifi-fallback.sh -i wlan1

# Run with custom hotspot settings
airdancer-wifi-fallback.sh -s "MyRaspberryPi" -p "mypassword123"

# Run with shorter timeout
airdancer-wifi-fallback.sh -t 60

# Run with debug logging
airdancer-wifi-fallback.sh -v
```

#### Hardware requirements

- One WiFi interface that supports AP mode (most modern adapters do)
- NetworkManager installed and running
- Root privileges or membership in the `netdev` group

#### How it works

1. Checks if WiFi is already connected, exits if so
2. Waits for NetworkManager to automatically connect to known networks
3. If no connection is established within the timeout period, enables hotspot mode
4. Hotspot remains active as a fallback for manual network configuration
5. Script exits after enabling hotspot (one-time operation)

### airdancer-ui

A web-based user interface for manual switch control.

#### Configuration file

```toml
# HTTP server configuration
listen-address = ""  # Leave empty to listen on all interfaces
listen-port = 8081

# API server configuration
api-base-url = "http://localhost:8080"
```

#### Command line options

- `--api-base-url string` - Base URL for the API server (default: "http://localhost:8080")
- `--config string` - Configuration file to use
- `--listen-address string` - Listen address for UI server (default: all interfaces)
- `--listen-port int` - Listen port for UI server (default: 8081)
- `--version` - Show version and exit

#### Example usage

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

#### Command line options

- `--spi-device string` - SPI device path (default: "/dev/spidev0.0")
- `--version` - Show version and exit
- `-h, --help` - Show help

#### Subcommands

- `read inputs` - Read current input pin states
- `read outputs` - Read current output pin states  
- `write pin:value` - Set output pins to specified values
- `reflect` - Continuously mirror input pins to output pins

#### Example usage

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

#### Pin Values

- `1`, `on`, `true` - Turn pin on
- `0`, `off`, `false` - Turn pin off

### gpiotest

A command-line tool for testing and controlling GPIO pins.

#### Command line options

- `--polarity string` - GPIO polarity: ActiveHigh or ActiveLow (default: "ActiveHigh")

#### Example usage

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

#### Pin values

- `1`, `on`, `true` - Turn pin on
- `0`, `off`, `false` - Turn pin off

### configvalidate

A configuration validation tool that validates configuration files for the airdancer programs.

#### Command line options

- `--config string` - Configuration file to validate
- `--type string` - Configuration type: `api`, `ui`, or `monitor`
- `-h, --help` - Show help
- `--version` - Show version and exit

#### Example usage

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

Use the provided `Makefile` to build all binaries:

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
```

## Installation

### System Installation

The `system/` directory contains systemd service files and configuration for system-wide installation:

```bash
# Install system files
cd system/
sudo make install

# Enable and start services
sudo systemctl enable airdancer.target
sudo systemctl start airdancer.target
```

## License

airdancer -- a system for controlling switches  
Copyright (C) 2025 Lars Kellogg-Stedman

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
