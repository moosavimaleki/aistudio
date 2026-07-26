"use strict";

(() => {
  // این worker صف داخلی را poll می‌کند و هر نتیجه را دقیقاً به همان job برمی‌گرداند.
  function startWorker({ chromeApi, factory, page, protocol, delay = defaultDelay }) {
    chromeApi.runtime.onConnect.addListener((port) => {
      if (port.name === protocol.keepAlivePort) port.onMessage.addListener(() => {});
    });

    async function poll() {
      for (;;) {
        try {
          const job = await factory.nextJob();
          if (!job) {
            await delay(500);
            continue;
          }
          try {
            await factory.complete(job.id, await page.execute(job));
          } catch (error) {
            await factory.complete(job.id, {
              error: error instanceof Error ? error.message : String(error),
            });
          }
        } catch (_error) {
          await delay(1_000);
        }
      }
    }

    poll();
  }

  const defaultDelay = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));
  const api = { startWorker };
  globalThis.AIStudioWorker = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
