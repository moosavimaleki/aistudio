#!/usr/bin/env python3
"""Manage an hourly local systemd-user cookie sync timer, with a numbered menu."""

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
NAME = "aistudio-session-sync"
CONFIG = Path(os.environ.get("XDG_CONFIG_HOME", Path.home() / ".config"))
UNITS = CONFIG / "systemd/user"
SETTINGS = CONFIG / NAME / "settings.json"
DEFAULT_TARGET = "root@152.228.145.18:/opt/services/aistudio-api"


def systemctl(*args, check=True):
    return subprocess.run(["systemctl", "--user", *args], check=check)


def validate(target, port):
    host, separator, directory = target.partition(":")
    if not separator or not re.fullmatch(r"(?:[\w][\w.-]*@)?[\w][\w.-]*", host, re.ASCII):
        raise ValueError("Target must be user@host:/absolute/project/path")
    if not directory.startswith("/") or directory == "/" or any(c in target for c in "\n\r\0"):
        raise ValueError("Remote project path must be absolute and not /")
    if not 1 <= port <= 65535:
        raise ValueError("SSH port must be between 1 and 65535")


def unit_quote(value):
    # systemd specifiers and ExecStart environment substitution are not shell quoting.
    value = str(value)
    if any(c in value for c in "\n\r\0"):
        raise ValueError("Newlines are not permitted in unit values")
    return '"' + value.replace("\\", "\\\\").replace('"', '\\"').replace("%", "%%").replace("$", "$$") + '"'


def render_service(settings):
    command = [shutil.which("bash") or "/bin/bash", str(ROOT / "scripts/sync_sessions_to_server.sh"),
               "--batch", "--ssh-port", str(settings["port"])]
    if settings["import_chatgpt"]:
        command.append("--import-chatgpt")
    command.append(settings["target"])
    path = os.environ.get("PATH", "/usr/local/bin:/usr/bin:/bin")
    # Environment= does not expand $, unlike ExecStart=.
    env_path = unit_quote("PATH=" + path).replace("$$", "$")
    env_python = unit_quote("AISTUDIO_SYNC_PYTHON=" + sys.executable).replace("$$", "$")
    return f"""[Unit]
Description=Sync local AI Studio and ChatGPT cookies to server

[Service]
Type=oneshot
Environment={env_path}
Environment={env_python}
ExecStart={' '.join(unit_quote(arg) for arg in command)}
TimeoutStartSec=45min
UMask=0077
"""


TIMER = f"""[Unit]
Description=Hourly AI Studio session sync

[Timer]
OnCalendar=hourly
Persistent=true
AccuracySec=1min
Unit={NAME}.service

[Install]
WantedBy=timers.target
"""


def install(args):
    if args.target is None:
        args.target = input(f"SSH target [{DEFAULT_TARGET}]: ").strip() or DEFAULT_TARGET
    if args.port is None:
        args.port = int(input("SSH port [22]: ").strip() or "22")
    validate(args.target, args.port)
    for program in ("bash", "ssh", "rsync", "flock"):
        if not shutil.which(program):
            raise ValueError(f"Missing dependency: {program}")
    if not args.no_import_chatgpt:
        # Use this interpreter at runtime, not a different systemd PATH Python.
        subprocess.run([sys.executable, "-c", "import yt_dlp.cookies"], check=True)
    settings = {"target": args.target, "port": args.port, "import_chatgpt": not args.no_import_chatgpt}
    service = render_service(settings)
    # No generation, cookie export, remote restart or SSH during installation.
    UNITS.mkdir(parents=True, exist_ok=True)
    SETTINGS.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    SETTINGS.write_text(json.dumps(settings, indent=2) + "\n")
    SETTINGS.chmod(0o600)
    (UNITS / f"{NAME}.service").write_text(service)
    (UNITS / f"{NAME}.timer").write_text(TIMER)
    systemctl("daemon-reload")
    variables = [key for key in ("DISPLAY", "WAYLAND_DISPLAY", "SSH_AUTH_SOCK", "DBUS_SESSION_BUS_ADDRESS")
                 if key in os.environ]
    if variables:
        systemctl("import-environment", *variables)
    systemctl("enable", "--now", f"{NAME}.timer")
    print("Installed hourly timer. Existing local Google cookie files will be uploaded.")
    print("ChatGPT cookies are refreshed from local Chrome unless --no-import-chatgpt is used.")
    print("Persistent=true may run a missed hour immediately. Run 'status' to see the next time.")


def perform(action, args):
    if action == "install":
        install(args)
    elif action == "run":
        systemctl("start", "--no-block", f"{NAME}.service")
        print("Started in background; use logs/status to inspect the result.")
    elif action == "restart":
        systemctl("restart", f"{NAME}.timer")
        print("Timer restarted; an active sync is not interrupted. Use run for an immediate sync.")
    elif action == "stop":
        systemctl("disable", "--now", f"{NAME}.timer")
        print("Scheduling disabled; any active sync is left to finish safely.")
    elif action == "status":
        systemctl("status", "--no-pager", f"{NAME}.timer", f"{NAME}.service", check=False)
        systemctl("list-timers", "--all", "--no-pager", f"{NAME}.timer", check=False)
    elif action == "logs":
        subprocess.run(["journalctl", "--user", "-u", f"{NAME}.service", "-n", "60", "--no-pager"], check=True)
    elif action == "uninstall":
        if input("Remove scheduling and settings? Cookies will NOT be removed. [y/N]: ").strip().lower() != "y":
            return
        active = subprocess.run(["systemctl", "--user", "is-active", "--quiet", f"{NAME}.service"])
        if active.returncode == 0:
            raise ValueError("Sync is running; wait for it to finish before uninstalling")
        systemctl("disable", "--now", f"{NAME}.timer", check=False)
        for path in (UNITS / f"{NAME}.timer", UNITS / f"{NAME}.service", SETTINGS):
            path.unlink(missing_ok=True)
        systemctl("daemon-reload")
        systemctl("reset-failed", f"{NAME}.service", check=False)
        print("Removed timer, service and settings. Cookies, project and server are unchanged.")


def main():
    actions = ("install", "run", "restart", "stop", "status", "logs", "uninstall")
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", nargs="?", choices=actions)
    parser.add_argument("--target")
    parser.add_argument("--port", type=int)
    parser.add_argument("--no-import-chatgpt", action="store_true")
    args = parser.parse_args()
    if not shutil.which("systemctl"):
        parser.error("Linux with systemd user services is required")
    if args.action:
        perform(args.action, args)
        return
    while True:
        print("\n1) نصب/به‌روزرسانی اجرای ساعتی\n2) اجرای فوری\n3) ری‌استارت تایمر"
              "\n4) توقف زمان‌بندی\n5) وضعیت و اجرای بعدی\n6) لاگ‌ها\n7) حذف سرویس\n0) خروج")
        choice = input("انتخاب: ").strip()
        if choice == "0":
            return
        if choice not in tuple(str(i) for i in range(1, 8)):
            print("گزینه نامعتبر است.")
            continue
        try:
            perform(actions[int(choice) - 1], args)
        except (ValueError, OSError, subprocess.CalledProcessError) as error:
            print(f"ERROR: {error}", file=sys.stderr)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError, EOFError) as error:
        raise SystemExit(f"ERROR: {error}") from error
