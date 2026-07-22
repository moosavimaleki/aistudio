"use strict";

(() => {
  // این مخزن فقط lifecycle مربوط به provider بومی صفحه را نگهداری می‌کند.
  function createProviderStore(root = globalThis, delay = defaultDelay) {
    const states = [];

    function install() {
      const namespace = root.botguard = root.botguard || {};
      const existingEntry = namespace.a;
      let runtimeEntry;

      Object.defineProperty(namespace, "a", {
        configurable: true,
        enumerable: true,
        get: () => runtimeEntry,
        set: (entry) => {
          runtimeEntry = typeof entry === "function" ? wrapEntry(entry) : entry;
        },
      });

      if (existingEntry !== undefined) namespace.a = existingEntry;
    }

    function wrapEntry(entry) {
      return function capturedRuntimeEntry(...args) {
        const onReady = args[1];
        if (typeof onReady === "function") args[1] = wrapReady(onReady);
        return entry.apply(this, args);
      };
    }

    function wrapReady(onReady) {
      return function capturedReady(snapshotFunction, cleanupFunction) {
        states.push({ snapshotFunction, cleanupFunction, readyAt: Date.now() });
        return onReady.apply(this, arguments);
      };
    }

    async function waitForReady(timeoutMs = 45_000) {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        const ready = states.filter((state) => typeof state?.snapshotFunction === "function");
        if (ready.length) {
          await waitForWarmup(ready);
          return ready;
        }
        await delay(100);
      }
      throw new Error("AI Studio native attestation provider did not initialize in container Chrome");
    }

    async function waitForWarmup(ready) {
      // این فاصله اجازه می‌دهد initialize بومی صفحه state داخلی provider را کامل کند.
      const newestReadyAt = Math.max(...ready.map((state) => state.readyAt));
      const remaining = 6_000 - (Date.now() - newestReadyAt);
      if (remaining > 0) await delay(remaining);
    }

    return { install, waitForReady };
  }

  const defaultDelay = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));
  const api = { createProviderStore };
  globalThis.AIStudioProviderStore = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
