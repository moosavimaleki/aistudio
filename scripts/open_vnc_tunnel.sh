#!/usr/bin/env bash
set -euo pipefail

# Keep VNC/noVNC private on the server and expose it only through SSH.
# Usage: scripts/open_vnc_tunnel.sh user@server [local_novnc_port] [local_vnc_port]

REMOTE="${1:-}"
SSH_PORT="${AISTUDIO_SSH_PORT:-22}"
LOCAL_NOVNC_PORT="${2:-7900}"
REMOTE_VNC_PORT="${AISTUDIO_REMOTE_VNC_PORT:-4314}"
LOCAL_VNC_PORT="${3:-4314}"

if [[ -z "$REMOTE" || "$REMOTE" == -* ]]; then
    if [[ "$REMOTE" == "-h" || "$REMOTE" == "--help" ]]; then
    printf 'Usage: AISTUDIO_SSH_PORT=9011 %s user@server [local_novnc_port] [local_vnc_port]\n' "$0"
        exit 0
    fi
    printf 'Usage: %s user@server [local_novnc_port] [local_vnc_port]\n' "$0" >&2
    exit 2
fi

case "$LOCAL_NOVNC_PORT:$LOCAL_VNC_PORT" in
    *[!0-9:]*|*:|:*)
        printf 'error: local ports must be numeric\n' >&2
        exit 2
        ;;
esac

printf 'noVNC: http://127.0.0.1:%s/vnc.html\n' "$LOCAL_NOVNC_PORT"
printf 'VNC:   127.0.0.1:%s\n' "$LOCAL_VNC_PORT"
printf 'Press Ctrl-C to close the tunnel.\n'

exec ssh -p "$SSH_PORT" -N -o ExitOnForwardFailure=yes \
    -L "127.0.0.1:${LOCAL_NOVNC_PORT}:127.0.0.1:7900" \
    -L "127.0.0.1:${LOCAL_VNC_PORT}:127.0.0.1:${REMOTE_VNC_PORT}" \
    "$REMOTE"
