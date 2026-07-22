"use strict";

(() => {
  // این سرویس فقط snapshot تازه می‌گیرد و دربارهٔ انتخاب provider تصمیم می‌گیرد.
  function createSnapshotService(store, delay = defaultDelay) {
    async function create(digest, providerIndex) {
      const states = await store.waitForReady();
      if (Number.isInteger(providerIndex)) {
        const state = states[providerIndex];
        if (!state) throw new Error(`Native provider ${providerIndex} is not available`);
        return {
          token: await freshSnapshot(state, digest),
          providerCount: states.length,
        };
      }

      const candidateTokens = await Promise.all(
        states.map((state) => freshSnapshot(state, digest)),
      );
      return {
        token: candidateTokens.at(-1),
        candidateTokens,
        providerCount: states.length,
      };
    }

    async function freshSnapshot(state, digest) {
      if (!state.primed) {
        // این snapshot اول فقط provider را آماده می‌کند و هیچ‌وقت به backend ارسال نمی‌شود.
        await snapshot(state, digest);
        state.primed = true;
        await delay(100);
      }
      return snapshot(state, digest);
    }

    function snapshot(state, digest) {
      return new Promise((resolve, reject) => {
        try {
          state.snapshotFunction((token) => {
            if (typeof token !== "string" || !token) {
              reject(new Error("Native provider returned an empty token"));
              return;
            }
            resolve(token);
          }, [{ content: digest }, undefined, undefined, undefined]);
        } catch (error) {
          reject(error);
        }
      });
    }

    return { create };
  }

  const defaultDelay = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));
  const api = { createSnapshotService };
  globalThis.AIStudioSnapshotService = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
