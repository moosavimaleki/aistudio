"use strict";

(() => {
  // این heartbeat فقط service worker را هنگام بازبودن صفحهٔ آزمایشگاه زنده نگه می‌دارد.
  function startKeepAlive(runtime, portName, intervalMs = 20_000) {
    let port;
    let timer;

    function stop() {
      if (timer) clearInterval(timer);
      timer = undefined;
      port = undefined;
    }

    try {
      port = runtime.connect({ name: portName });
      port.onDisconnect.addListener(stop);
      timer = setInterval(() => {
        try {
          if (!runtime.id || !port) return stop();
          port.postMessage({ type: "heartbeat" });
        } catch (_error) {
          stop();
        }
      }, intervalMs);
    } catch (_error) {
      stop();
    }

    return stop;
  }

  const api = { startKeepAlive };
  globalThis.AIStudioKeepAlive = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
