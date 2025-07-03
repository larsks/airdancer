# Integration Testing for Hardware Drivers

This document describes the integration testing strategy for Airdancer's hardware-dependent drivers (PiFace and GPIO). These tests require actual hardware and cannot run on standard development machines.

## Test Organization

### Test Types

1. **Unit Tests** - Run on any machine, use mocks/stubs
2. **Integration Tests** - Require actual hardware, use build tags
3. **End-to-End Tests** - Full system tests with real hardware

### Build Tags

Integration tests use Go build tags to separate them from unit tests:

- `//go:build integration && piface` - PiFace hardware tests
- `//go:build integration && gpio` - GPIO hardware tests

This allows running different test suites:

```bash
# Unit tests only (default)
go test ./...

# PiFace integration tests only
go test -tags="integration,piface" ./internal/piface/...

# GPIO integration tests only
go test -tags="integration,gpio" ./internal/gpio/...

# All integration tests
go test -tags="integration,piface" ./internal/piface/...
go test -tags="integration,gpio" ./internal/gpio/...
```

## Running Tests Locally

### Prerequisites

For local integration testing, you need:

1. **Hardware**: Raspberry Pi or compatible board
2. **Drivers**: PiFace Digital I/O board and/or GPIO access
3. **Permissions**: Member of `gpio` group or root access
4. **Go**: Version 1.19 or later

### Setup

1. Add your user to the gpio group:
   ```bash
   sudo usermod -a -G gpio $USER
   # Log out and back in for changes to take effect
   ```

2. Enable SPI interface (for PiFace):
   ```bash
   sudo raspi-config
   # Interface Options -> SPI -> Enable
   ```

3. Verify hardware access:
   ```bash
   ls -l /dev/spidev*     # Should show SPI devices
   ls -l /sys/class/gpio  # Should show GPIO interface
   ```

### Running Integration Tests

Use the provided scripts for organized test execution:

```bash
# Run all integration tests
./scripts/test-integration.sh

# Run specific driver tests
./scripts/test-integration.sh piface
./scripts/test-integration.sh gpio

# Run with additional options
./scripts/test-integration.sh -v -r all  # verbose + race detector
./scripts/test-integration.sh -s piface  # short mode (skip long tests)
```

The script will:
- Check hardware availability
- Verify permissions
- Run appropriate integration tests
- Report results with clear error messages

## Remote Testing

For development machines without hardware, use remote execution:

### SSH-Based Remote Testing

```bash
# Run tests on remote Raspberry Pi
./scripts/test-remote.sh pi@192.168.1.100

# Run specific driver tests
./scripts/test-remote.sh -d piface -u pi rpi.local

# Use SSH key and custom port  
./scripts/test-remote.sh -k ~/.ssh/id_rsa -p 2222 -v 10.0.0.5
```

The remote script will:
1. Upload code to remote host
2. Check remote environment (Go, hardware, permissions)
3. Execute integration tests
4. Clean up temporary files
5. Report results

### Remote Host Requirements

- SSH access with key-based authentication
- Go 1.19+ installed
- Hardware drivers available
- User in `gpio` group or root access
- rsync available for file transfer

## CI/CD Integration

### GitHub Actions

The project includes a comprehensive CI/CD pipeline:

#### Standard Tests (GitHub Hosted Runners)
- Unit tests with coverage reporting
- Build tests for multiple platforms (Linux, macOS, ARM)
- Static analysis and linting
- Documentation validation

#### Hardware Tests (Self-Hosted Runners)
- Run on actual Raspberry Pi hardware
- Automatically detect available hardware
- Skip tests gracefully if hardware unavailable
- Only run on main branch or with `test-hardware` label

### Self-Hosted Runner Setup

1. **Prepare Hardware**:
   ```bash
   # Install GitHub Actions runner on Raspberry Pi
   cd /home/pi
   curl -o actions-runner-linux-arm64-2.311.0.tar.gz -L https://github.com/actions/runner/releases/download/v2.311.0/actions-runner-linux-arm64-2.311.0.tar.gz
   tar xzf ./actions-runner-linux-arm64-2.311.0.tar.gz
   ```

2. **Configure Runner**:
   ```bash
   # Follow GitHub's instructions to register runner
   ./config.sh --url https://github.com/yourusername/airdancer --token YOUR_TOKEN
   
   # Add labels for hardware identification
   ./config.sh --labels raspberry-pi,gpio,piface
   ```

3. **Install Dependencies**:
   ```bash
   # Install Go
   sudo apt update
   sudo apt install golang-go
   
   # Set up permissions
   sudo usermod -a -G gpio github-runner
   ```

4. **Start Runner**:
   ```bash
   sudo ./svc.sh install
   sudo ./svc.sh start
   ```

### Triggering Hardware Tests

Hardware tests run automatically on:
- Pushes to `main` branch
- Pull requests with `test-hardware` label

To manually trigger hardware tests on a PR:
1. Add the `test-hardware` label to the pull request
2. Tests will run on the next push or when manually triggered

## Test Structure

### Integration Test Files

```
internal/
├── piface/
│   ├── piface_test.go              # Unit tests
│   └── piface_integration_test.go  # Hardware integration tests
└── gpio/
    ├── gpio_test.go                # Unit tests  
    └── gpio_integration_test.go    # Hardware integration tests
```

### Test Content

Integration tests verify:

1. **Hardware Detection**:
   - SPI device availability (/dev/spidev*)
   - GPIO character device availability (/dev/gpiochip*)
   - Permission validation

2. **Basic Operations**:
   - Driver initialization
   - Switch enumeration
   - Turn on/off operations
   - State reporting

3. **Individual Control**:
   - Single switch operations
   - Toggle functionality
   - State persistence

4. **Error Handling**:
   - Invalid operations
   - Hardware failures
   - Permission errors

### Test Data and Configuration

Integration tests use environment variables for configuration:

- `AIRDANCER_TEST_GPIO_PINS`: GPIO pins to test (default: "18,19,20,21")
- `AIRDANCER_TEST_SPI_DEVICE`: SPI device path (default: "/dev/spidev0.0")

## Troubleshooting

### Common Issues

1. **Permission Denied**:
   ```bash
   sudo usermod -a -G gpio $USER
   # Log out and back in
   ```

2. **SPI Not Available**:
   ```bash
   sudo raspi-config
   # Enable SPI in Interface Options
   sudo reboot
   ```

3. **GPIO Access Denied**:
   ```bash
   ls -l /dev/gpiochip*
   # Check ownership and permissions of GPIO character devices
   ```

4. **Tests Timeout or Hang**:
   - Hardware may be in inconsistent state
   - Power cycle the board
   - Check physical connections

### Debug Mode

Run tests with verbose logging:

```bash
./scripts/test-integration.sh -v all
```

Use short mode to skip long-running tests:

```bash
./scripts/test-integration.sh -s all
```

### Manual Hardware Verification

Verify hardware manually before running tests:

```bash
# Test GPIO character device access (modern interface)
ls -l /dev/gpiochip*

# Test SPI access (if using PiFace)
ls -l /dev/spidev*

# Note: The old sysfs GPIO interface (/sys/class/gpio) is deprecated
# Modern GPIO access uses character devices (/dev/gpiochip*)
```

## Best Practices

1. **Always Clean Up**: Integration tests should leave hardware in clean state
2. **Use Timeouts**: Hardware operations should have reasonable timeouts
3. **Check Prerequisites**: Verify hardware availability before running tests
4. **Graceful Degradation**: Skip tests if hardware unavailable, don't fail
5. **Logging**: Provide clear error messages for debugging
6. **Isolation**: Tests should not interfere with each other
7. **Documentation**: Keep this document updated with changes 