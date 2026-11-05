#!/bin/bash
set -euo pipefail

pause() {
  sleep "$1" &
  wait $!
}

trap 'kill "$pid" 2>/dev/null; exit 0' TERM INT

while true; do
  wireproxy -c /etc/wireproxy.conf &
  pid=$!
  pause 30
  failures=0
  while kill -0 "$pid" 2>/dev/null; do
    if timeout 15 tunnel-check; then failures=0; else failures=$((failures + 1)); fi
    if [ "$failures" -ge 3 ]; then
      echo "watchdog: tunnel not ready, restarting wireproxy on a new source port" >&2
      kill "$pid"
      break
    fi
    pause 10
  done
  wait "$pid" || true
done
