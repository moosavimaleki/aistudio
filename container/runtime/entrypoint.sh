#!/usr/bin/env bash
set -euo pipefail

# Never let a containerized Chrome renderer feed a multi-gigabyte core to the
# host's core_pattern handler. Chrome's small Crashpad minidumps remain usable.
ulimit -c 0

case "${CHATGPT_MODE:-}" in
  browser) export CHROME_HEADLESS=false ;;
  headless) export CHROME_HEADLESS=true ;;
  direct|"") ;;
  *) echo "CHATGPT_MODE must be browser, headless or direct" >&2; exit 1 ;;
esac

browser="${CHROME_EXECUTABLE:-/opt/cloakbrowser/chrome}"
if [[ ! -x "$browser" ]]; then
  echo "CloakBrowser executable is missing: $browser" >&2
  exit 1
fi

runtime_dir="${AISTUDIO_RUNTIME_DIR:-/app/runtime/state}"
sudo mkdir -p "$runtime_dir"
sudo chown -R seluser:seluser "$runtime_dir"
sudo chmod -R u+rwX "$runtime_dir"

touch /home/seluser/.Xauthority
export XAUTHORITY=/home/seluser/.Xauthority
export DISPLAY=:99
export CHROME_DISPLAY_SERVICES=true
case "${CHROME_HEADLESS:-false}" in
  1|[tT][rR][uU][eE]) export CHROME_DISPLAY_SERVICES=false ;;
esac

exec /opt/venv/bin/supervisord -c /app/supervisor/supervisord.conf
