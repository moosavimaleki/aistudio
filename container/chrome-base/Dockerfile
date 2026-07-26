# syntax=docker/dockerfile:1.6
FROM selenium/standalone-chrome:latest

# Use non-interactive mode for apt
ENV DEBIAN_FRONTEND=noninteractive

###############################################################################
# Install system packages and X11 utilities
###############################################################################
USER root
RUN --mount=type=cache,id=apt-cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,id=apt-lib,target=/var/lib/apt,sharing=locked \
    apt-get update && \
    apt-get install -y --no-install-recommends \
      xvfb \
      xclip \
      x11vnc \
      fluxbox \
      net-tools \
      netcat-openbsd \
      scrot \
      python3-tk \
      libxtst6 \
      libpng-dev \
      wget \
      x11-utils \
      tigervnc-standalone-server \
      gnome-screenshot \
      tigervnc-tools \
      tigervnc-common \
      gedit && \
      xterm xfonts-base xfonts-100dpi xfonts-75dpi xclip pyperclip python3-tk && \
      eterm && \
      xfonts-scalable xfonts-terminess-ttf && \
      pyperclip && \
    rm -rf /var/lib/apt/lists/*

###############################################################################
# Install Python dependencies
###############################################################################
COPY --link requirements.txt /tmp/requirements.txt
RUN --mount=type=cache,id=pip-cache,target=/root/.cache/pip \
    pip install --no-cache-dir -r /tmp/requirements.txt

###############################################################################
# Prepare screenshot directory and set ownership
###############################################################################
RUN mkdir -p /home/seluser/screenshots && \
    chown -R seluser:seluser /app /home/seluser/screenshots && \
    chown -R seluser:seluser /home/seluser/.local/

###############################################################################
# COPY --link and configure entrypoint script
###############################################################################
COPY --link docker-entrypoint.sh /opt/bin/docker-entrypoint.sh
RUN chmod +x /opt/bin/docker-entrypoint.sh && \
    chown seluser:seluser /opt/bin/docker-entrypoint.sh

###############################################################################
# Switch to non-root user
###############################################################################
USER seluser

# Ensure Xauthority file exists for Xlib authentication
RUN mkdir -p /home/seluser && touch /home/seluser/.Xauthority

# Set working directory
WORKDIR /app

###############################################################################
# Expose ports for Selenium Grid, VNC, and noVNC
###############################################################################
EXPOSE 4444
EXPOSE 5900
EXPOSE 7900

###############################################################################
# Entrypoint
###############################################################################
ENTRYPOINT ["/opt/bin/docker-entrypoint.sh"]
