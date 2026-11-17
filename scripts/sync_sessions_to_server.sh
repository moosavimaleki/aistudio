#!/usr/bin/env bash
set -euo pipefail

# Copy local staging sessions to a remote Compose checkout.
# Usage: scripts/sync_sessions_to_server.sh user@server:/srv/aistudio-api

PROJECT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
REMOTE_TARGET=""
SSH_PORT="${AISTUDIO_SSH_PORT:-22}"
RESTART=1
IMPORT_CHATGPT=0
PRUNE=0
BATCH=0

usage() {
    cat <<'EOF'
Usage:
  scripts/sync_sessions_to_server.sh [options] user@server:/absolute/project/path

Options:
  --import-chatgpt  Export healthy local Chrome ChatGPT sessions first.
  --prune           Remove remote *.txt files not present locally.
  --no-restart      Do not restart the remote aistudio service.
  --ssh-port PORT   SSH port (default: 22, or AISTUDIO_SSH_PORT).
  --batch           Noninteractive SSH; fail instead of asking for a password.
  -h, --help        Show this help.

The source is the local COOKIES/ and CHATGPT_COOKIES/ directories. Cookie
contents are never printed. The remote target must be the directory containing
docker-compose.yml and the two cookie directories.
EOF
}

die() {
    printf 'error: %s\n' "$1" >&2
    exit 1
}

shell_quote() {
    local value="$1"
    value=${value//\'/\'\\\'\'}
    printf "'%s'" "$value"
}

count_cookie_files() {
    local directory="$1"
    find "$directory" -maxdepth 1 -type f -name '*.txt' -readable -print 2>/dev/null | wc -l
}

while (($# > 0)); do
    case "$1" in
        --import-chatgpt)
            IMPORT_CHATGPT=1
            shift
            ;;
        --prune)
            PRUNE=1
            shift
            ;;
        --batch)
            BATCH=1
            shift
            ;;
        --no-restart)
            RESTART=0
            shift
            ;;
        --ssh-port)
            (($# >= 2)) || die "--ssh-port requires a value"
            SSH_PORT="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        -* )
            die "unknown option: $1"
            ;;
        *)
            [[ -z "$REMOTE_TARGET" ]] || die "only one remote target is allowed"
            REMOTE_TARGET="$1"
            shift
            ;;
    esac
done

[[ -n "$REMOTE_TARGET" ]] || { usage >&2; exit 2; }
[[ "$REMOTE_TARGET" == *:* ]] || die "target must look like user@server:/absolute/project/path"

REMOTE_HOST="${REMOTE_TARGET%%:*}"
REMOTE_DIR="${REMOTE_TARGET#*:}"
[[ "$REMOTE_HOST" =~ ^([a-zA-Z0-9_][a-zA-Z0-9_.-]*@)?[a-zA-Z0-9_][a-zA-Z0-9_.-]*$ ]] || die "invalid SSH host"
[[ "$REMOTE_DIR" == /* ]] || die "remote project path must be absolute"
[[ "$REMOTE_DIR" != / && "$REMOTE_DIR" != *$'\n'* && "$REMOTE_DIR" != *$'\r'* ]] || die "unsafe remote path"
[[ "$SSH_PORT" =~ ^[0-9]+$ ]] || die "SSH port must be numeric"
((10#$SSH_PORT >= 1 && 10#$SSH_PORT <= 65535)) || die "SSH port out of range"
for tool in ssh rsync flock; do
    command -v "$tool" >/dev/null || die "$tool is required"
done
mkdir -p "$PROJECT_ROOT/.local"
exec 9>"$PROJECT_ROOT/.local/session-sync.lock"
flock -n 9 || { printf 'Session sync is already running; skipping.\n'; exit 0; }
SSH_OPTIONS=(-p "$SSH_PORT" -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=2)
if ((BATCH)); then
    SSH_OPTIONS+=(-o BatchMode=yes -o StrictHostKeyChecking=yes)
fi
printf -v RSYNC_SSH '%q ' ssh "${SSH_OPTIONS[@]}"

# Validate the destination before doing a potentially slow Chrome export.
remote_root_q="$(shell_quote "$REMOTE_DIR")"
ssh "${SSH_OPTIONS[@]}" "$REMOTE_HOST" "test -f $remote_root_q/docker-compose.yml" ||
    die "No docker-compose.yml at $REMOTE_TARGET; check the deployed project path (usually /opt/services/aistudio-api)"

for directory in COOKIES CHATGPT_COOKIES; do
    [[ -d "$PROJECT_ROOT/$directory" ]] || die "missing local directory: $directory"
done

if ((IMPORT_CHATGPT)); then
    import_python="${AISTUDIO_SYNC_PYTHON:-python3}"
    command -v "$import_python" >/dev/null 2>&1 || die "Python is required for --import-chatgpt"
    printf 'Importing healthy local ChatGPT sessions...\n'
    if ! "$import_python" "$PROJECT_ROOT/scripts/import_chatgpt_cookies.py" --replace; then
        die "Chrome import failed; no cookies uploaded and remote service was not restarted"
    fi
fi

google_count="$(count_cookie_files "$PROJECT_ROOT/COOKIES")"
chatgpt_count="$(count_cookie_files "$PROJECT_ROOT/CHATGPT_COOKIES")"
((google_count + chatgpt_count > 0)) || die "no local .txt cookie files found"

remote_root_q="$(shell_quote "$REMOTE_DIR")"
# shellcheck disable=SC2029 # Intentionally expand locally shell-quoted remote paths.
ssh "${SSH_OPTIONS[@]}" "$REMOTE_HOST" "set -eu; test -f $remote_root_q/docker-compose.yml; umask 077; mkdir -p -- $remote_root_q/COOKIES $remote_root_q/CHATGPT_COOKIES"

sync_directory() {
    local name="$1"
    local source="$PROJECT_ROOT/$name/"
    local destination="$REMOTE_TARGET/$name/"
    local delete_flag=()
    local unreadable=()
    local remote_owner
    # Keep freshly uploaded files owned by the cookie directory's runtime user.
    # Otherwise a root SSH upload creates root-owned files the container cannot read.
    # shellcheck disable=SC2029 # remote_root_q is quoted, name is a fixed directory name.
    remote_owner="$(ssh "${SSH_OPTIONS[@]}" "$REMOTE_HOST" "stat -c '%u:%g' -- $remote_root_q/$name")"
    [[ "$remote_owner" =~ ^[0-9]+:[0-9]+$ ]] || die "cannot determine remote cookie owner"
    while IFS= read -r -d '' file; do
        unreadable+=(--exclude "${file##*/}")
    done < <(find "$source" -maxdepth 1 -type f -name '*.txt' ! -readable -print0 2>/dev/null)
    if ((${#unreadable[@]} > 0)); then
        printf 'SKIP %s unreadable cookie file(s) due to local permissions.\n' "$name"
    fi
    if ((PRUNE)); then
        if ((${#unreadable[@]} == 0)); then
            delete_flag=(--delete)
        else
            printf 'WARN %s: prune disabled for this directory.\n' "$name"
        fi
    fi
    rsync --archive --compress --protect-args --delay-updates \
        --chown="$remote_owner" -e "$RSYNC_SSH" \
        --chmod=F640,D770 "${delete_flag[@]}" "${unreadable[@]}" \
        --include='*.txt' --exclude='*' \
        "$source" "$destination"
}

sync_directory COOKIES
sync_directory CHATGPT_COOKIES

if ((RESTART)); then
    printf 'Resetting remote Chrome profiles...\n'
    # shellcheck disable=SC2029 # Intentionally pass the locally quoted project path.
    ssh "${SSH_OPTIONS[@]}" "$REMOTE_HOST" "cd -- $remote_root_q && docker compose up -d --no-build --force-recreate aistudio"
fi

printf 'Synced %s AI Studio and %s ChatGPT cookie file(s) to %s.\n' \
    "$google_count" "$chatgpt_count" "$REMOTE_TARGET"
if ((RESTART == 0)); then
    printf 'Remote service was not restarted; run: ssh %s "cd %s && docker compose restart aistudio"\n' \
        "$REMOTE_HOST" "$REMOTE_DIR"
fi
