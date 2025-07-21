#!/bin/bash

# 1: Active connection
# 2: IP address
# 3: Temperature
# 4: API status
# 4: Switches

get_switch_states() {
  curl -sf localhost:8080/switch/all | jq -r '.data.switches|to_entries[]|[.key, .value.state]|join(",")' | awk -F, '
  {
    if ($2 == "on")
      printf("X")
    else
      printf("_")
  }
  END {printf("\n")}
  '
}

while :; do
  active_connection=$(nmcli -g name,device c show --active | awk -F: '$2 == "wlan0" {print $1}')
  addr=$(ip -j addr show wlan0 | jq -r '.[].addr_info[]|select(.family == "inet").local')
  api=$(systemctl is-active airdancer-api)
  temperature=$(vcgencmd measure_temp | cut -f2 -d=)
  switches=$(get_switch_states)
  cat <<EOF | display1306
CN: ${active_connection:-???}
IP: ${addr:-???}
TP: ${temperature}
AP: ${api}
SW: ${switches}
EOF

  sleep 10
done
