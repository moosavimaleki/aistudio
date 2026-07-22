#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
service_dir="$(cd -- "$script_dir/../.." && pwd)"

exec docker compose -f "$service_dir/docker-compose.yml" --project-directory "$service_dir" \
  exec -T browser-interface \
  /opt/bin/run-python-client "$@"
