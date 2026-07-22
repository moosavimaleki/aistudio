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
