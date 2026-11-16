#!/usr/bin/env bash
set -euo pipefail

case "${CHATGPT_MODE:-}" in
  browser) export CHROME_HEADLESS=false ;;
  headless) export CHROME_HEADLESS=true ;;
  direct|"") ;;
  *) echo "CHATGPT_MODE must be browser, headless or direct" >&2; exit 1 ;;
esac


runtime_dir="${AISTUDIO_RUNTIME_DIR:-/app/runtime/state}"
sudo mkdir -p "$runtime_dir"
sudo chown -R seluser:seluser "$runtime_dir"
sudo chmod -R u+rwX "$runtime_dir"

touch /home/seluser/.Xauthority
export XAUTHORITY=/home/seluser/.Xauthority
export DISPLAY=:99

exec /opt/venv/bin/supervisord -c /app/supervisor/supervisord.conf
