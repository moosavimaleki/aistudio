#!/usr/bin/env bash
set -euo pipefail

# این launcher فقط workerهای FastAPI واحد gencontent را اجرا می‌کند.
export PYTHONPATH="/app${PYTHONPATH:+:$PYTHONPATH}"
exec /opt/venv/bin/python -m uvicorn gencontent.app:app \
  --host 0.0.0.0 \
  --port "${GENCONTENT_PORT:-8000}" \
  --workers "${GENCONTENT_WORKERS:-2}"
