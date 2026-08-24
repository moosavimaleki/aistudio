"use strict";

(() => {
  function createChannel(pageWindow, origin, protocol, timeoutMs = 180_000) {
    const pending = new Map();

    pageWindow.addEventListener("message", (event) => {
      if (event.source !== pageWindow || event.origin !== origin) return;
      const capture = pending.get(event.data?.jobId);
      if (!capture) return;
      if (event.data.source === protocol.chatCaptureReadySource) {
        capture.arm();
        return;
      }
      if (event.data.source === protocol.chatResponseSource) capture.finish(event.data);
    });

    function capture(jobId, options = {}) {
      let arm;
      let finish;
      let resultTimeout;
      let readyTimeout;
      const ready = new Promise((resolve, reject) => {
        readyTimeout = setTimeout(() => {
          pending.delete(jobId);
          reject(new Error("ChatGPT page hook did not become ready"));
        }, Math.min(timeoutMs, 10_000));
        arm = () => {
          clearTimeout(readyTimeout);
          resolve();
        };
      });
      const result = new Promise((resolve) => {
        resultTimeout = setTimeout(() => {
          pending.delete(jobId);
          resolve({ error: "ChatGPT page timed out while waiting for its native response" });
        }, timeoutMs);
        finish = (value) => {
          arm();
          clearTimeout(resultTimeout);
          pending.delete(jobId);
          resolve(value);
        };
      });
      const cancel = () => {
        arm();
        clearTimeout(resultTimeout);
        pending.delete(jobId);
      };
      pending.set(jobId, { arm, finish });
      pageWindow.postMessage({ source: protocol.chatRequestSource, jobId, ...options }, origin);
      return { ready, result, cancel };
    }

    return { capture };
  }

  const api = { createChannel };
  globalThis.ChatGPTChannel = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
