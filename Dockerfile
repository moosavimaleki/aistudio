FROM golang:1.22-bookworm AS go-builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY go ./go
RUN CGO_ENABLED=0 go build -mod=mod -o /out/browser-interface ./go/cmd/browser-interface \
    && CGO_ENABLED=0 go build -mod=mod -o /out/gencontent ./go/cmd/gencontent

FROM moosavimaleki/ai-chrome-base:latest AS extension-builder

USER root
WORKDIR /build
COPY config ./config
COPY src/extension ./extension
RUN /opt/venv/bin/python /build/extension/build.py \
    && rm -f /build/extension/build.py \
    && rm -rf /build/extension/__pycache__

FROM moosavimaleki/ai-chrome-base:latest

USER root
WORKDIR /app

COPY config ./config
COPY --from=go-builder /out/browser-interface /app/bin/browser-interface
COPY --from=go-builder /out/gencontent /app/bin/gencontent
COPY --from=extension-builder /build/extension ./extension
COPY src/selenium/entrypoint.sh ./selenium/entrypoint.sh
COPY src/selenium/scripts ./selenium/scripts
COPY scripts/run-browser-interface /opt/bin/run-browser-interface
COPY scripts/run-gencontent /opt/bin/run-gencontent

RUN chmod +x /app/selenium/entrypoint.sh /app/selenium/scripts/*.sh \
        /opt/bin/run-browser-interface /opt/bin/run-gencontent \
    && mkdir -p /app/selenium/runtime /app/browser-profiles \
    && chown -R seluser:seluser /app /opt/bin/run-browser-interface \
        /opt/bin/run-gencontent

USER seluser
ENV PORT=3345
ENV CHROME_EXECUTABLE=/usr/bin/google-chrome
ENV CHROME_RUNTIME_DIR=/tmp/aistudio-browsers
ENV EXTENSION_SOURCE_DIR=/app/extension
ENV SELENIUM_RUNTIME_DIR=/app/selenium/runtime

EXPOSE 3345 8000
ENTRYPOINT ["/app/selenium/entrypoint.sh"]
