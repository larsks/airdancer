#!/bin/bash

# WiFi Hotspot Fallback Script for Airdancer
# This script monitors WiFi connectivity and automatically enables hotspot mode
# when no known networks are available.

set -euo pipefail

# Configuration defaults (can be overridden by config file)
AIRDANCER_WIFI_INTERFACE="${AIRDANCER_WIFI_INTERFACE:-wlan0}"
AIRDANCER_HOTSPOT_SSID="${AIRDANCER_HOTSPOT_SSID:-AirdancerSetup}"
AIRDANCER_HOTSPOT_PASSWORD="${AIRDANCER_HOTSPOT_PASSWORD:-airdancer123}"
AIRDANCER_CONNECTION_TIMEOUT="${AIRDANCER_CONNECTION_TIMEOUT:-120}"
AIRDANCER_CHECK_INTERVAL="${AIRDANCER_CHECK_INTERVAL:-5}"
AIRDANCER_CONFIG_FILE="${AIRDANCER_CONFIG_FILE:-/etc/airdancer/wifi-fallback.conf}"
AIRDANCER_LOG_LEVEL="${AIRDANCER_LOG_LEVEL:-INFO}"

# Load configuration file if it exists
if [[ -f "$AIRDANCER_CONFIG_FILE" ]]; then
  # shellcheck source=/dev/null
  source "$AIRDANCER_CONFIG_FILE"
fi

# Logging functions
log() {
  local level="$1"
  shift
  local message="$*"
  local timestamp
  timestamp=$(date '+%Y-%m-%d %H:%M:%S')
  echo "[$timestamp] [$level] $message" >&2
}

log_info() {
  [[ "$AIRDANCER_LOG_LEVEL" =~ ^(DEBUG|INFO)$ ]] && log "INFO" "$@" || :
}

log_warn() {
  [[ "$AIRDANCER_LOG_LEVEL" =~ ^(DEBUG|INFO|WARN)$ ]] && log "WARN" "$@" || :
}

log_error() {
  log "ERROR" "$@"
}

log_debug() {
  [[ "$AIRDANCER_LOG_LEVEL" == "DEBUG" ]] && log "DEBUG" "$@" || :
}

# Check if we're running as root or with appropriate privileges
check_privileges() {
  if [[ $EUID -ne 0 ]] && ! groups "$USER" | grep -q netdev; then
    log_error "This script requires root privileges or membership in the netdev group"
    exit 1
  fi
}

# Check if NetworkManager is running
check_networkmanager() {
  if ! systemctl is-active --quiet NetworkManager; then
    log_error "NetworkManager is not running"
    exit 1
  fi
}

# Check if required interface exists
check_interface_exists() {
  if ! ip link show "$AIRDANCER_WIFI_INTERFACE" &>/dev/null; then
    log_error "WiFi interface $AIRDANCER_WIFI_INTERFACE not found"
    log_info "Available WiFi interfaces:"
    iw dev 2>/dev/null | awk '/Interface/ {print "  " $2}' | sort
    return 1
  fi
}

check_interface_ap() {
  # Check if interface supports AP mode
  local phy_name
  if phy_name=$(iw dev "$AIRDANCER_WIFI_INTERFACE" info 2>/dev/null | grep wiphy | awk '{print $2}'); then
    if ! iw phy "phy${phy_name}" info 2>/dev/null | grep -A 10 "Supported interface modes" | grep -q "AP"; then
      log_warn "Interface $AIRDANCER_WIFI_INTERFACE (phy${phy_name}) may not support AP mode"
    else
      log_debug "Interface $AIRDANCER_WIFI_INTERFACE (phy${phy_name}) supports AP mode"
    fi
  else
    log_warn "Could not determine phy for interface $AIRDANCER_WIFI_INTERFACE"
  fi
}

# Check if WiFi is connected to a network
is_wifi_connected() {
  local state
  state=$(nmcli -t -f GENERAL.STATE device show "$AIRDANCER_WIFI_INTERFACE" 2>/dev/null | cut -d: -f2)
  [[ "$state" == "100 (connected)" ]]
}

# Wait for NetworkManager to establish a connection
wait_for_connection() {
  local timeout="$1"
  local elapsed=0

  log_info "Waiting up to ${timeout}s for NetworkManager to establish connection..."

  while [[ $elapsed -lt $timeout ]]; do
    if is_wifi_connected; then
      local connection_name
      connection_name=$(nmcli -t -f GENERAL.CONNECTION device show "$AIRDANCER_WIFI_INTERFACE" 2>/dev/null | cut -d: -f2)
      log_info "Network connection established to '$connection_name'"
      return 0
    fi

    sleep "$AIRDANCER_CHECK_INTERVAL"
    ((elapsed += AIRDANCER_CHECK_INTERVAL))
    log_debug "Waiting for connection... (${elapsed}s/${timeout}s)"
  done

  log_info "No network connection established within ${timeout}s"
  return 1
}

# Enable hotspot mode
enable_hotspot() {
  log_info "Enabling hotspot mode on $AIRDANCER_WIFI_INTERFACE..."

  # Disconnect any existing connections
  nmcli device disconnect "$AIRDANCER_WIFI_INTERFACE" 2>/dev/null || true

  # Delete existing hotspot connection if it exists
  nmcli connection delete "airdancer-hotspot" 2>/dev/null || true

  # Create and activate hotspot
  if nmcli device wifi hotspot \
    ifname "$AIRDANCER_WIFI_INTERFACE" \
    con-name "airdancer-hotspot" \
    ssid "$AIRDANCER_HOTSPOT_SSID" \
    password "$AIRDANCER_HOTSPOT_PASSWORD" \
    band bg; then
    log_info "Hotspot '$AIRDANCER_HOTSPOT_SSID' enabled successfully on $AIRDANCER_WIFI_INTERFACE"
    return 0
  else
    log_error "Failed to enable hotspot"
    return 1
  fi
}

# Disable hotspot mode
disable_hotspot() {
  log_info "Disabling hotspot mode..."

  # Disconnect and delete hotspot connection
  if nmcli connection show "airdancer-hotspot" &>/dev/null; then
    nmcli connection down "airdancer-hotspot" 2>/dev/null || true
    nmcli connection delete "airdancer-hotspot" 2>/dev/null || true
    log_info "Hotspot disabled successfully"
  else
    log_debug "No hotspot connection to disable"
  fi
}

# Main function - simplified approach
run_wifi_fallback() {
  log_info "Starting WiFi fallback monitor on interface: $AIRDANCER_WIFI_INTERFACE"
  log_info "Hotspot fallback: SSID=$AIRDANCER_HOTSPOT_SSID, Password=$AIRDANCER_HOTSPOT_PASSWORD"
  log_info "Connection timeout: ${AIRDANCER_CONNECTION_TIMEOUT}s"

  # Check if already connected
  if is_wifi_connected; then
    local connection_name
    connection_name=$(nmcli -t -f GENERAL.CONNECTION device show "$AIRDANCER_WIFI_INTERFACE" 2>/dev/null | cut -d: -f2)
    log_info "Already connected to '$connection_name', no hotspot needed"
    return 0
  fi

  # Wait for NetworkManager to establish a connection
  if wait_for_connection "$AIRDANCER_CONNECTION_TIMEOUT"; then
    log_info "NetworkManager successfully established connection, no hotspot needed"
    return 0
  fi

  # No connection established, enable hotspot as fallback
  log_info "No network connection available, enabling hotspot mode as fallback..."
  if enable_hotspot; then
    log_info "Hotspot enabled successfully. Connect to '$AIRDANCER_HOTSPOT_SSID' to configure network settings."
    return 0
  else
    log_error "Failed to enable hotspot"
    return 1
  fi
}

# Signal handlers
cleanup() {
  log_info "Received signal, shutting down..."
  disable_hotspot
  exit 0
}

# Set up signal handlers
trap cleanup INT TERM

# Usage information
usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

WiFi Hotspot Fallback Script for Airdancer

This script monitors WiFi connectivity and automatically enables hotspot mode
when no known networks are available.

OPTIONS:
    -c, --config FILE       Configuration file (default: $AIRDANCER_CONFIG_FILE)
    -i, --interface IFACE   WiFi interface (default: $AIRDANCER_WIFI_INTERFACE)
    -s, --ssid SSID         Hotspot SSID (default: $AIRDANCER_HOTSPOT_SSID)
    -p, --password PASS     Hotspot password (default: $AIRDANCER_HOTSPOT_PASSWORD)
    -t, --timeout SECONDS   Connection timeout (default: $AIRDANCER_CONNECTION_TIMEOUT)
    -v, --verbose           Enable debug logging
    --help                  Show this help message

CONFIGURATION:
    Configuration can be set via environment variables or config file.
    Config file should contain shell variable assignments, e.g.:
    
    AIRDANCER_WIFI_INTERFACE=wlan0
    AIRDANCER_HOTSPOT_SSID=MyHotspot
    AIRDANCER_HOTSPOT_PASSWORD=MyPassword
    AIRDANCER_CONNECTION_TIMEOUT=120
    AIRDANCER_CHECK_INTERVAL=5
    AIRDANCER_LOG_LEVEL=INFO

EXAMPLES:
    # Run with default settings
    $0
    
    # Run with custom interface
    $0 -i wlan0
    
    # Run with custom hotspot settings
    $0 -s "AirdancerSetup" -p "mypassword123"
    
    # Run with custom timeout
    $0 -t 60
    
    # Run with debug logging
    $0 -v

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
  -c | --config)
    AIRDANCER_CONFIG_FILE="$2"
    shift 2
    ;;
  -i | --interface)
    AIRDANCER_WIFI_INTERFACE="$2"
    shift 2
    ;;
  -s | --ssid)
    AIRDANCER_HOTSPOT_SSID="$2"
    shift 2
    ;;
  -p | --password)
    AIRDANCER_HOTSPOT_PASSWORD="$2"
    shift 2
    ;;
  -t | --timeout)
    AIRDANCER_CONNECTION_TIMEOUT="$2"
    shift 2
    ;;
  -v | --verbose)
    AIRDANCER_LOG_LEVEL="DEBUG"
    shift
    ;;
  --help)
    usage
    exit 0
    ;;
  *)
    log_error "Unknown option: $1"
    usage
    exit 1
    ;;
  esac
done

# Main execution
main() {
  log_info "Airdancer WiFi Hotspot Fallback starting..."

  check_privileges
  check_networkmanager
  while ! check_interface_exists; do
    sleep 5
  done
  check_interface_ap

  # Run the fallback logic (one-time operation)
  run_wifi_fallback
}

# Run main function
main "$@"

exit 0
