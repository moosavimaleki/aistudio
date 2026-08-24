"use strict";

(() => {
  function createChannel(pageWindow, origin, protocol, timeoutMs = 180_000) {
    const pending = new Map();

    pageWindow.addEventListener("message", (event) => {
      if (event.source !== pageWindow || event.origin !== origin) return;
      if (event.data?.source !== protocol.chatResponseSource) return;
      const finish = pending.get(event.data.jobId);
      if (finish) finish(event.data);
    });

    function capture(jobId, options = {}) {
      let cancel;
      const result = new Promise((resolve) => {
        const timeout = setTimeout(() => {
          pending.delete(jobId);
          resolve({ error: "ChatGPT page timed out while waiting for its native response" });
        }, timeoutMs);
        cancel = () => {
          clearTimeout(timeout);
          pending.delete(jobId);
        };
        pending.set(jobId, (value) => {
          cancel();
          resolve(value);
        });
      });
      pageWindow.postMessage({ source: protocol.chatRequestSource, jobId, ...options }, origin);
      return { result, cancel };
    }

    return { capture };
  }

  const api = { createChannel };
  globalThis.ChatGPTChannel = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
