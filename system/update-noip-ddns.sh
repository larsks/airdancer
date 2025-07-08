#!/bin/bash

NOIP_CONFIG_FILE=${NOIP_CONFIG_FILE:-/etc/airdancer/noip.conf}
NOIP_UPDATE_URL=${NOIP_UPDATE_URL:-http://dynupdate.no-ip.com/nic/update}
NOIP_LOG_LEVEL=${NOIP_LOG_LEVEL:-INFO}

get_interface_ip() {
  ip -j addr show | jq -r --arg ifname "$ifname" '.[]|select(.ifname == $ifname).addr_info[]|select(.family == "inet").local'
}

log() {
  local level="$1"
  shift
  local message="$*"
  local timestamp
  timestamp=$(date '+%Y-%m-%d %H:%M:%S')
  echo "[$timestamp] [$level]: $0: $message" >&2
}

log_fatal() {
  log_error "$@"
  exit 1
}

log_error() {
  log "ERROR" "$@"
}

log_warning() {
  [[ "$NOIP_LOG_LEVEL" =~ ^(DEBUG|INFO|WARNING|WARN)$ ]] && log "WARNING" "$@" || :
}

log_info() {
  [[ "$NOIP_LOG_LEVEL" =~ ^(DEBUG|INFO)$ ]] && log "INFO" "$@" || :
}

log_debug() {
  [[ "$NOIP_LOG_LEVEL" = DEBUG ]] && log "DEBUG" "$@" || :
  log "DEBUG" "$@"
}

ifname=$1
ifstate=$2

# Load configuration file if it exists
if [[ -f "$NOIP_CONFIG_FILE" ]]; then
  . "$NOIP_CONFIG_FILE"
fi

for required in NOIP_USERNAME NOIP_PASSWORD NOIP_HOSTNAME; do
  if [[ -z "${!required}" ]]; then
    log_fatal "missing required configuration variable $required"
  fi
done

if [[ $ifstate = up ]]; then
  if [[ -z $NOIP_ADDRESS ]]; then
    NOIP_ADDRESS=$(get_interface_ip)
    [[ -z $NOIP_ADDRESS ]] && log_fatal "unable to determine ip address"
  fi

  log_info "update address for $NOIP_HOSTNAME to $NOIP_ADDRESS"
  curl -Sf -u "${NOIP_USERNAME}:${NOIP_PASSWORD}" \
    "${NOIP_UPDATE_URL}?hostname=${NOIP_HOSTNAME}&myip=${NOIP_ADDRESS}" ||
    log_fatal "failed to update address for $NOIP_HOSTNAME"
fi
