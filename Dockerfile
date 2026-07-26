ARG CHROME_BASE_IMAGE=moosavimaleki/ai-chrome-base:latest

FROM golang:1.22-bookworm AS go-builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY config ./config
COPY assets/extension ./assets/extension
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -mod=mod -o /out/browser-interface ./cmd/browser-interface \
    && CGO_ENABLED=0 go build -mod=mod -o /out/gencontent ./cmd/gencontent \
    && go run -mod=mod ./cmd/build-extension \
        -source /build/assets/extension \
        -config /build/config/upstream.yaml

FROM ${CHROME_BASE_IMAGE}

ARG VERSION=dev
LABEL org.opencontainers.image.title="AI Studio API" \
      org.opencontainers.image.description="Go gateway backed by container Chrome for the AI Studio staging lab" \
      org.opencontainers.image.version="${VERSION}"

USER root
WORKDIR /app

COPY config ./config
COPY --from=go-builder /out/browser-interface /app/bin/browser-interface
COPY --from=go-builder /out/gencontent /app/bin/gencontent
COPY --from=go-builder /build/assets/extension ./extension
COPY container/runtime/entrypoint.sh ./runtime/entrypoint.sh
COPY container/supervisor ./supervisor

RUN rm -f /app/product_collector.py /app/selenium_script.py \
    && chmod +x /app/runtime/entrypoint.sh /app/supervisor/* \
    && mkdir -p /app/runtime/state /app/browser-profiles \
    && chown -R seluser:seluser /app

USER seluser
ENV PORT=3345
ENV CHROME_EXECUTABLE=/usr/bin/google-chrome
ENV CHROME_RUNTIME_DIR=/tmp/aistudio-browsers
ENV EXTENSION_SOURCE_DIR=/app/extension
ENV AISTUDIO_RUNTIME_DIR=/app/runtime/state

EXPOSE 3345 8000
HEALTHCHECK --interval=5s --timeout=3s --retries=20 \
    CMD ["/app/supervisor/healthcheck"]
ENTRYPOINT ["/app/runtime/entrypoint.sh"]
