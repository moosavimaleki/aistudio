"use strict";

(() => {
  // این نام‌ها قرارداد ثابت بین MAIN world، content script و service worker هستند.
  const protocol = Object.freeze({
    requestSource: "aistudio-container-token-bridge-extension",
    responseSource: "aistudio-container-token-bridge-page",
    jobMessage: "AISTUDIO_CONTAINER_TOKEN_JOB",
    keepAlivePort: "aistudio-container-bridge-keepalive",
  });

  globalThis.AIStudioBridgeProtocol = protocol;
  if (typeof module !== "undefined" && module.exports) module.exports = protocol;
})();


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


"use strict";

(() => {
  // این کانال پاسخ‌های MAIN world را با job منتظر در isolated world جفت می‌کند.
  function createPageChannel(pageWindow, origin, protocol, timeoutMs = 50_000) {
    const pending = new Map();

    pageWindow.addEventListener("message", (event) => {
      if (event.source !== pageWindow || event.origin !== origin) return;
      if (event.data?.source !== protocol.responseSource) return;
      const finish = pending.get(event.data.jobId);
      if (finish) finish(event.data);
    });

    function request(job, sendResponse) {
      const timeout = setTimeout(() => {
        pending.delete(job.jobId);
        sendResponse({ error: "Container page bridge timed out while waiting for native provider" });
      }, timeoutMs);

      pending.set(job.jobId, (result) => {
        clearTimeout(timeout);
        pending.delete(job.jobId);
        sendResponse(result);
      });

      pageWindow.postMessage({
        source: protocol.requestSource,
        jobId: job.jobId,
        digest: job.digest,
        authUser: job.authUser,
        providerIndex: job.providerIndex,
      }, origin);
    }

    return { request };
  }

  const api = { createPageChannel };
  globalThis.AIStudioPageChannel = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();


"use strict";

(() => {
  const protocol = globalThis.AIStudioBridgeProtocol;
  const channel = globalThis.AIStudioPageChannel.createPageChannel(
    window,
    location.origin,
    protocol,
  );

  // این اتصال باعث می‌شود polling در service worker بین jobها متوقف نشود.
  globalThis.AIStudioKeepAlive.startKeepAlive(
    chrome.runtime,
    protocol.keepAlivePort,
  );

  // این listener تنها پیام job معتبر را به MAIN world صفحه عبور می‌دهد.
  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message?.type !== protocol.jobMessage) return false;
    channel.request(message, sendResponse);
    return true;
  });
})();

