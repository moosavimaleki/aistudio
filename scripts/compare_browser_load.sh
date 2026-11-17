#!/usr/bin/env bash
set -euo pipefail

# A disposable, read-only page-load comparison. No cookie import or challenge click.
cloak_binary="${1:?usage: compare_browser_load.sh /path/to/cloakbrowser/chrome [url]}"
target_url="${2:-https://chatgpt.com/}"
chrome_binary="${CHROME_BINARY:-/usr/bin/google-chrome}"
extension_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../assets/extension" && pwd)"

for binary in "$chrome_binary" "$cloak_binary"; do
  if [[ ! -x "$binary" ]]; then
    echo "Browser executable is missing: $binary" >&2
    exit 2
  fi
done

trial_dir="$(mktemp -d)"
browser_pid=""
cleanup() {
  if [[ -n "$browser_pid" ]]; then
    kill -- "-$browser_pid" 2>/dev/null || true
    wait "$browser_pid" 2>/dev/null || true
  fi
  rm -r -- "$trial_dir"
}
trap cleanup EXIT

run_trial() {
  local label="$1" binary="$2" profile="$trial_dir/$1"
  local started port title page_url elapsed sample state
  local -a browser_args=(
    --no-sandbox --disable-gpu --no-first-run --no-default-browser-check
    --remote-debugging-address=127.0.0.1 --remote-debugging-port=0
    --user-data-dir="$profile"
    --disable-extensions-except="$extension_dir" --load-extension="$extension_dir"
  )
  if [[ -n "${SMOKE_PROXY_URL:-}" ]]; then
    browser_args+=(--proxy-server="$SMOKE_PROXY_URL" '--proxy-bypass-list=127.0.0.1;localhost')
  fi
  mkdir -p "$profile"
  started="$(date +%s)"
  setsid timeout 50s xvfb-run -a "$binary" \
    "${browser_args[@]}" \
    "$target_url" >"$trial_dir/$label.log" 2>&1 &
  browser_pid="$!"

  port=""
  for _ in $(seq 1 60); do
    if [[ -s "$profile/DevToolsActivePort" ]]; then
      port="$(head -n 1 "$profile/DevToolsActivePort")"
      break
    fi
    sleep 0.5
  done
  if [[ -z "$port" ]]; then
    printf '%s\tcdp-unavailable\t-\t-\n' "$label"
  else
    title=""
    state="not-loaded"
    for _ in $(seq 1 60); do
      sample="$(curl -fsS --max-time 2 "http://127.0.0.1:$port/json/list" 2>/dev/null | jq -r --arg target "$target_url" '[.[] | select(.type == "page" and (.url | startswith($target)))] | first | [.title // "", .url // ""] | @tsv' 2>/dev/null || true)"
      title="${sample%%$'\t'*}"
      page_url="${sample#*$'\t'}"
      if [[ "$title" == *'Just a moment'* || "$page_url" == *'__cf_chl'* ]]; then
        state="challenge"
      elif [[ "$title" == *'ChatGPT:'* ]]; then
        state="page-title-loaded"
        break
      fi
      sleep 0.5
    done
    elapsed="$(($(date +%s) - started))"
    printf '%s\t%s\t%ss\t%s\n' "$label" "$state" "$elapsed" "${title:-no page title}"
  fi
  kill -- "-$browser_pid" 2>/dev/null || true
  wait "$browser_pid" 2>/dev/null || true
  browser_pid=""
}

printf 'browser\tstate\telapsed\ttitle\n'
if [[ "${SMOKE_ORDER:-}" == "cloak-first" ]]; then
  run_trial cloakbrowser "$cloak_binary"
  run_trial chrome "$chrome_binary"
else
  run_trial chrome "$chrome_binary"
  run_trial cloakbrowser "$cloak_binary"
fi
