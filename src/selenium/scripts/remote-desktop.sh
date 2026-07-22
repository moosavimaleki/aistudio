#!/usr/bin/env bash

# این تابع VNC و noVNC را برای مشاهدهٔ اختیاری Chromeهای آزمایشگاه اجرا می‌کند.
start_remote_desktop() {
  local runtime_dir="$1"
  x11vnc -display :99 -nopw -forever -shared -rfbport 5900 \
    > "$runtime_dir/x11vnc.log" 2>&1 &
  VNC_PID=$!

  /opt/venv/bin/websockify --web=/opt/bin/noVNC 7900 127.0.0.1:5900 \
    > "$runtime_dir/novnc.log" 2>&1 &
  NOVNC_PID=$!
}
