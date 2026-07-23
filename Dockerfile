FROM moosavimaleki/ai-chrome-base:latest

USER root
WORKDIR /app

COPY requirements.txt /tmp/requirements.txt
RUN /opt/venv/bin/pip install --no-cache-dir -r /tmp/requirements.txt \
    && rm /tmp/requirements.txt

COPY config ./config
COPY src/shared ./shared
COPY src/extension ./extension
RUN /opt/venv/bin/python /app/extension/build.py
COPY src/selenium ./selenium
COPY src/lab_metrics ./lab_metrics
COPY src/browser_interface ./browser_interface
COPY src/aistudio_client ./aistudio_client
COPY src/browser_interface/run.sh /opt/bin/run-browser-interface
COPY src/gencontent/run.sh /opt/bin/run-gencontent
COPY src/scripts/container-python-client.sh /opt/bin/run-python-client
COPY src/gencontent ./gencontent

RUN chmod +x /app/selenium/entrypoint.sh /app/selenium/scripts/*.sh \
        /opt/bin/run-browser-interface /opt/bin/run-python-client /opt/bin/run-gencontent \
    && mkdir -p /app/selenium/runtime /app/browser-profiles \
    && chown -R seluser:seluser /app /opt/bin/run-browser-interface \
        /opt/bin/run-python-client /opt/bin/run-gencontent

USER seluser
ENV PORT=3345
ENV CHROME_EXECUTABLE=/usr/bin/google-chrome
ENV CHROME_RUNTIME_DIR=/tmp/aistudio-browsers
ENV EXTENSION_SOURCE_DIR=/app/extension
ENV SELENIUM_RUNTIME_DIR=/app/selenium/runtime
ENV PYTHONPATH=/app

EXPOSE 3345 8000
ENTRYPOINT ["/app/selenium/entrypoint.sh"]
