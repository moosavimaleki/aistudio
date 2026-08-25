"use strict";

(() => {
  // این heartbeat فقط service worker را هنگام بازبودن صفحهٔ آزمایشگاه زنده نگه می‌دارد.
  function startKeepAlive(runtime, portName, intervalMs = 20_000, retryMs = 1_000) {
    let port;
    let timer;
    let retryTimer;
    let stopped = false;

    function clearConnection() {
      if (timer) clearInterval(timer);
      timer = undefined;
      port = undefined;
    }

    function reconnect() {
      clearConnection();
      if (stopped || retryTimer) return;
      retryTimer = setTimeout(() => {
        retryTimer = undefined;
        connect();
      }, retryMs);
    }

    function connect() {
      if (stopped || !runtime.id) return;
      try {
        const connection = runtime.connect({ name: portName });
        port = connection;
        connection.onDisconnect.addListener(() => {
          if (port === connection) reconnect();
        });
        timer = setInterval(() => {
          try {
            connection.postMessage({ type: "heartbeat" });
          } catch (_error) {
            reconnect();
          }
        }, intervalMs);
      } catch (_error) {
        reconnect();
      }
    }

    function stop() {
      stopped = true;
      clearConnection();
      if (retryTimer) clearTimeout(retryTimer);
      retryTimer = undefined;
    }

    connect();
    return stop;
  }

  const api = { startKeepAlive };
  globalThis.AIStudioKeepAlive = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
