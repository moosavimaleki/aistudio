#!/usr/bin/env bash
set -euo pipefail

# این launcher فقط برای اجرای تشخیصی client داخل image آزمایشگاه است.
export PYTHONPATH="/app${PYTHONPATH:+:$PYTHONPATH}"
exec /opt/venv/bin/python -u -m aistudio_client.cli "$@"
