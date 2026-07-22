#!/usr/bin/env bash
set -euo pipefail

# این launcher فقط process مربوط به FastAPI واحد browser_interface را اجرا می‌کند.
export PYTHONPATH="/app${PYTHONPATH:+:$PYTHONPATH}"
exec /opt/venv/bin/python -m uvicorn browser_interface.app:app \
  --host 0.0.0.0 \
  --port "${PORT:-3345}" \
  --no-access-log \
  --workers 1
