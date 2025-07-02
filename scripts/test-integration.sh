#!/bin/bash

set -e

# Integration test runner for hardware-dependent tests
# Usage: ./scripts/test-integration.sh [driver] [options]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Default values
DRIVER=""
VERBOSE=false
RACE=false
SHORT=false

# Function to show usage
usage() {
  cat <<EOF
Usage: $0 [OPTIONS] [DRIVER]

Run integration tests for hardware-dependent drivers.

DRIVERS:
    piface      Run PiFace integration tests
    gpio        Run GPIO integration tests
    all         Run all integration tests

OPTIONS:
    -v, --verbose       Run tests in verbose mode
    -r, --race         Run tests with race detector
    -s, --short        Run tests in short mode (skips long-running tests)
    -h, --help         Show this help message

ENVIRONMENT VARIABLES:
    AIRDANCER_TEST_GPIO_PINS    Comma-separated list of GPIO pins to test (default: "23,24")
    AIRDANCER_TEST_SPI_DEVICE   SPI device path for PiFace tests (default: "/dev/spidev0.0")

EXAMPLES:
    $0 piface                   # Run PiFace tests only
    $0 gpio -v                  # Run GPIO tests with verbose output
    $0 all --race               # Run all integration tests with race detector
    $0 piface -s                # Run PiFace tests in short mode

NOTES:
    - These tests require actual hardware to run
    - Tests will be skipped if hardware is not detected
    - Run with appropriate permissions (usually requires sudo or gpio group membership)
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  -v | --verbose)
    VERBOSE=true
    shift
    ;;
  -r | --race)
    RACE=true
    shift
    ;;
  -s | --short)
    SHORT=true
    shift
    ;;
  -h | --help)
    usage
    exit 0
    ;;
  piface | gpio | all)
    DRIVER="$1"
    shift
    ;;
  *)
    echo "Unknown option: $1"
    usage
    exit 1
    ;;
  esac
done

# Default to 'all' if no driver specified
if [[ -z "$DRIVER" ]]; then
  DRIVER="all"
fi

# Build test arguments
TEST_ARGS="-tags=integration"

if [[ "$VERBOSE" == "true" ]]; then
  TEST_ARGS="$TEST_ARGS -v"
fi

if [[ "$RACE" == "true" ]]; then
  TEST_ARGS="$TEST_ARGS -race"
fi

if [[ "$SHORT" == "true" ]]; then
  TEST_ARGS="$TEST_ARGS -short"
fi

# Change to project directory
cd "$PROJECT_DIR"

echo "Running integration tests for: $DRIVER"
echo "Test arguments: $TEST_ARGS"
echo

# Function to run tests for a specific driver
run_driver_tests() {
  local driver=$1
  local build_tags="integration,$driver"

  echo "=== Running $driver integration tests ==="
  echo "Build tags: $build_tags"
  echo

  # Set driver-specific build tags and run tests
  if ! go test $TEST_ARGS -tags="$build_tags" "./internal/$driver/..."; then
    echo "ERROR: $driver integration tests failed"
    return 1
  fi

  echo "$driver integration tests completed successfully"
  echo
}

# Function to check hardware prerequisites
check_prerequisites() {
  echo "=== Checking Prerequisites ==="

  # Check if running as root or with appropriate permissions
  if [[ $EUID -eq 0 ]]; then
    echo "✓ Running as root - should have hardware access"
  elif groups | grep -q "\bgpio\b"; then
    echo "✓ User is in gpio group - should have GPIO access"
  else
    echo "⚠ WARNING: Not running as root and not in gpio group"
    echo "  Hardware tests may fail due to permission issues"
    echo "  Consider running: sudo usermod -a -G gpio \$USER"
  fi

  # Check for common hardware interfaces
  if [[ -e "/dev/spidev0.0" ]]; then
    echo "✓ SPI device /dev/spidev0.0 detected"
  else
    echo "⚠ SPI device /dev/spidev0.0 not found - PiFace tests may fail"
  fi

  if [[ -d "/sys/class/gpio" ]]; then
    echo "✓ GPIO sysfs interface detected"
  else
    echo "⚠ GPIO sysfs interface not found - GPIO tests may fail"
  fi

  echo
}

# Run prerequisite checks
check_prerequisites

# Run tests based on driver selection
case $DRIVER in
piface)
  run_driver_tests "piface"
  ;;
gpio)
  run_driver_tests "gpio"
  ;;
all)
  run_driver_tests "piface"
  run_driver_tests "gpio"
  ;;
*)
  echo "Unknown driver: $DRIVER"
  usage
  exit 1
  ;;
esac

echo "All integration tests completed successfully!"

