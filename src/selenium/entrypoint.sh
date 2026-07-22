#!/usr/bin/env bash
set -euo pipefail

source /app/selenium/scripts/display.sh
source /app/selenium/scripts/remote-desktop.sh

runtime_dir="${SELENIUM_RUNTIME_DIR:-/app/selenium/runtime}"
sudo mkdir -p "$runtime_dir"
sudo chown -R seluser:seluser "$runtime_dir"
sudo chmod -R u+rwX "$runtime_dir"

touch /home/seluser/.Xauthority
export XAUTHORITY=/home/seluser/.Xauthority

start_display "$runtime_dir"
start_remote_desktop "$runtime_dir"

# این cleanup تمام processهای زیرساخت نمایشی و API را با هم متوقف می‌کند.
cleanup() {
  if [ -n "${API_PID:-}" ]; then kill "$API_PID" 2>/dev/null || true; fi
  kill "$NOVNC_PID" "$VNC_PID" "$DISPLAY_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

/opt/bin/run-browser-interface &
API_PID=$!
wait "$API_PID"
