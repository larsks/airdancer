#!/bin/bash

set -e

# Remote integration test runner
# Usage: ./scripts/test-remote.sh [options] <remote-host>

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Default values
REMOTE_HOST=""
REMOTE_USER=""
REMOTE_PATH="/tmp/airdancer"
DRIVER="all"
VERBOSE=false
RACE=false
SHORT=false
SSH_KEY=""
SSH_PORT="22"

# Function to show usage
usage() {
    cat << EOF
Usage: $0 [OPTIONS] <remote-host>

Run integration tests on remote hardware via SSH.

ARGUMENTS:
    remote-host         Remote host to run tests on (IP address or hostname)

OPTIONS:
    -u, --user USER     SSH username (default: current user)
    -p, --port PORT     SSH port (default: 22)
    -k, --key KEY       SSH private key file
    -d, --driver DRIVER Driver to test: piface, gpio, all (default: all)
    -v, --verbose       Run tests in verbose mode
    -r, --race          Run tests with race detector
    -s, --short         Run tests in short mode
    --path PATH         Remote path to upload code (default: /tmp/airdancer)
    -h, --help          Show this help message

EXAMPLES:
    $0 192.168.1.100                           # Run all tests on Raspberry Pi
    $0 -u pi -d piface rpi.local               # Run PiFace tests as user 'pi'
    $0 -k ~/.ssh/id_rsa -v 10.0.0.5           # Use specific SSH key with verbose output
    $0 --port 2222 --driver gpio my-pi.com    # Use custom SSH port for GPIO tests

REQUIREMENTS:
    - SSH access to remote host
    - Go installed on remote host
    - Hardware drivers available on remote host
    - Appropriate permissions for hardware access

NOTES:
    - Code will be uploaded to remote host temporarily
    - Remote host should have Go 1.19+ installed
    - Tests require hardware permissions (gpio group or root)
EOF
}

# Function to check dependencies
check_dependencies() {
    if ! command -v ssh >/dev/null 2>&1; then
        echo "ERROR: SSH client not found"
        exit 1
    fi
    
    if ! command -v rsync >/dev/null 2>&1; then
        echo "ERROR: rsync not found (required for file transfer)"
        exit 1
    fi
    
    if ! command -v go >/dev/null 2>&1; then
        echo "ERROR: Go not found (required for building)"
        exit 1
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--user)
            REMOTE_USER="$2"
            shift 2
            ;;
        -p|--port)
            SSH_PORT="$2"
            shift 2
            ;;
        -k|--key)
            SSH_KEY="$2"
            shift 2
            ;;
        -d|--driver)
            DRIVER="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -r|--race)
            RACE=true
            shift
            ;;
        -s|--short)
            SHORT=true
            shift
            ;;
        --path)
            REMOTE_PATH="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -*)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
        *)
            if [[ -z "$REMOTE_HOST" ]]; then
                REMOTE_HOST="$1"
            else
                echo "Multiple hosts not supported: $1"
                usage
                exit 1
            fi
            shift
            ;;
    esac
done

# Validate required arguments
if [[ -z "$REMOTE_HOST" ]]; then
    echo "ERROR: Remote host is required"
    usage
    exit 1
fi

# Build SSH command
SSH_CMD="ssh"
if [[ -n "$SSH_KEY" ]]; then
    SSH_CMD="$SSH_CMD -i $SSH_KEY"
fi
if [[ -n "$SSH_PORT" ]]; then
    SSH_CMD="$SSH_CMD -p $SSH_PORT"
fi
if [[ -n "$REMOTE_USER" ]]; then
    SSH_TARGET="$REMOTE_USER@$REMOTE_HOST"
else
    SSH_TARGET="$REMOTE_HOST"
fi

# Build rsync command
RSYNC_CMD="rsync -avz --delete"
if [[ -n "$SSH_KEY" ]]; then
    RSYNC_CMD="$RSYNC_CMD -e 'ssh -i $SSH_KEY -p $SSH_PORT'"
elif [[ -n "$SSH_PORT" ]] && [[ "$SSH_PORT" != "22" ]]; then
    RSYNC_CMD="$RSYNC_CMD -e 'ssh -p $SSH_PORT'"
fi

# Function to run command on remote host
remote_exec() {
    echo "Running on $SSH_TARGET: $*"
    $SSH_CMD "$SSH_TARGET" "$@"
}

# Function to upload code to remote host
upload_code() {
    echo "=== Uploading code to $SSH_TARGET:$REMOTE_PATH ==="
    
    # Create remote directory
    remote_exec "mkdir -p $REMOTE_PATH"
    
    # Upload source code (exclude build artifacts and git)
    eval "$RSYNC_CMD" \
        --exclude='.git/' \
        --exclude='bin/' \
        --exclude='*.log' \
        --exclude='.vscode/' \
        "$PROJECT_DIR/" \
        "$SSH_TARGET:$REMOTE_PATH/"
    
    echo "Code upload completed"
}

# Function to check remote environment
check_remote_environment() {
    echo "=== Checking remote environment ==="
    
    # Check Go installation
    if ! remote_exec "command -v go >/dev/null 2>&1"; then
        echo "ERROR: Go not found on remote host"
        echo "Please install Go 1.19+ on $REMOTE_HOST"
        exit 1
    fi
    
    GO_VERSION=$(remote_exec "go version" | cut -d' ' -f3)
    echo "✓ Go found: $GO_VERSION"
    
    # Check hardware interfaces
    echo "Checking hardware availability..."
    
    if remote_exec "test -e /dev/spidev0.0"; then
        echo "✓ SPI device detected"
    else
        echo "⚠ SPI device not found - PiFace tests may be skipped"
    fi
    
    if remote_exec "test -d /sys/class/gpio"; then
        echo "✓ GPIO interface detected"
    else
        echo "⚠ GPIO interface not found - GPIO tests may be skipped"
    fi
    
    # Check permissions
    echo "Checking permissions..."
    if remote_exec "groups | grep -q gpio || test \$EUID -eq 0"; then
        echo "✓ Hardware permissions available"
    else
        echo "⚠ Limited hardware permissions - tests may fail"
        echo "  Consider adding user to gpio group or running as root"
    fi
}

# Function to run remote tests
run_remote_tests() {
    echo "=== Running integration tests on $SSH_TARGET ==="
    
    # Build test arguments
    local test_args=""
    
    if [[ "$VERBOSE" == "true" ]]; then
        test_args="$test_args -v"
    fi
    
    if [[ "$RACE" == "true" ]]; then
        test_args="$test_args -r"
    fi
    
    if [[ "$SHORT" == "true" ]]; then
        test_args="$test_args -s"
    fi
    
    # Run the integration test script remotely
    remote_exec "cd $REMOTE_PATH && ./scripts/test-integration.sh $test_args $DRIVER"
}

# Function to cleanup remote files
cleanup_remote() {
    echo "=== Cleaning up remote files ==="
    remote_exec "rm -rf $REMOTE_PATH"
    echo "Remote cleanup completed"
}

# Main execution
main() {
    check_dependencies
    
    echo "Starting remote integration tests..."
    echo "Remote host: $SSH_TARGET"
    echo "Remote path: $REMOTE_PATH"
    echo "Driver: $DRIVER"
    echo
    
    # Upload code and run tests
    upload_code
    check_remote_environment
    run_remote_tests
    
    # Cleanup on success
    cleanup_remote
    
    echo
    echo "Remote integration tests completed successfully!"
}

# Trap to cleanup on exit/error
trap 'echo "Cleaning up..."; cleanup_remote || true' EXIT

# Run main function
main 