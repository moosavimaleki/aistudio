#!/usr/bin/env bash

# این تابع Xvfb را بالا می‌آورد و فقط بعد از آماده‌شدن display برمی‌گردد.
start_display() {
  local runtime_dir="$1"
  rm -f /tmp/.X99-lock /tmp/.X11-unix/X99
  export DISPLAY=:99

  Xvfb :99 -screen 0 1280x900x24 -nolisten tcp -ac \
    > "$runtime_dir/xvfb.log" 2>&1 &
  DISPLAY_PID=$!

  for _attempt in $(seq 1 100); do
    if xdpyinfo -display :99 >/dev/null 2>&1; then return 0; fi
    if ! kill -0 "$DISPLAY_PID" 2>/dev/null; then
      tail -100 "$runtime_dir/xvfb.log" >&2 || true
      return 1
    fi
    sleep 0.1
  done
  echo "Xvfb display did not become ready" >&2
  return 1
}
