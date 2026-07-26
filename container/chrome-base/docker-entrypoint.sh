#!/usr/bin/env bash
set -euo pipefail

sudo chown -R seluser:seluser /home/seluser/screenshots
sudo chmod -R 777 /home/seluser/screenshots

if [ -d /home/seluser/.config/google-chrome ]; then
    sudo chown -R seluser:seluser /home/seluser/.config/google-chrome
    sudo chmod -R u+rwX /home/seluser/.config/google-chrome
    mkdir -p "/home/seluser/.config/google-chrome/Crash Reports/new"
fi

if [ "$#" -gt 0 ]; then
    echo "▶️  Launching custom command: $*"
    exec "$@"
fi

###############################################################################
# 1) Start Selenium Grid (spawns Xvfb + Fluxbox + x11vnc + noVNC)
###############################################################################
echo "🚀  Starting Selenium Grid services…"
/opt/bin/entry_point.sh &
GRID_PID=$!

###############################################################################
# 2) Wait for Selenium Grid to come up on 4444
###############################################################################
#echo "⏳  Waiting for Selenium Grid on port 4444…"
#until nc -z localhost 4444; do sleep 1; done
#echo "✅  Selenium Grid is up"

###############################################################################
# 3) Wait for the built-in x11vnc to be listening on 5900
###############################################################################
echo "⏳  Waiting for x11vnc on port 5900…"
until nc -z localhost 5900; do sleep 1; done
echo "✅  x11vnc is listening on 5900"

###############################################################################
# 4) Setup Xauthority and DISPLAY
###############################################################################
XAUTH="$HOME/.Xauthority"
touch "$XAUTH"
export XAUTHORITY="$XAUTH"
export DISPLAY="127.0.0.1:99"
echo "🔧  DISPLAY set to $DISPLAY"

###############################################################################
# 5) Manual Mode vs Automation Mode
###############################################################################
if [ "${MANUAL_LOGIN_MODE:-false}" = "true" ]; then
    echo "🛠  [MANUAL_LOGIN_MODE] Automation bypassed."
    echo "💡  Connect to VNC and login to Chrome manually."
    echo "📂  Chrome data will be persisted in your mounted volume."
    tail -f /dev/null
else
    echo "⌛  Waiting for PyAutoGUI import..."
    # (Simplified check for brevity)
    python3 -c "import pyautogui; print('✅ PyAutoGUI ready')" || true

    echo "▶️  Launching Python script: /app/main.py"
    exec python3 -u /app/main.py "$@"
fi
