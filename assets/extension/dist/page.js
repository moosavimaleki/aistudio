globalThis.AISTUDIO_UPSTREAM_CONFIG = {"apiKeyProperty":"WIu0Nc","attestationEntrypoint":"a","attestationNamespace":"botguard","attestationProperty":"UsvuEb","digestProperty":"content","runtimeGlobal":"WIZ_global_data","visitIdProperty":"teM9xe"};

"use strict";

(() => {
  // این نام‌ها قرارداد ثابت بین MAIN world، content script و service worker هستند.
  const protocol = Object.freeze({
    requestSource: "aistudio-container-token-bridge-extension",
    responseSource: "aistudio-container-token-bridge-page",
    jobMessage: "AISTUDIO_CONTAINER_TOKEN_JOB",
    chatJobKind: "chatgpt.generate",
    chatImageJobKind: "chatgpt.generate_image",
    chatDirectJobKind: "chatgpt.prepare_direct",
    chatJobMessage: "CHATGPT_CONTAINER_CHAT_JOB",
    chatReadyMessage: "CHATGPT_CONTAINER_READY",
    chatRequestSource: "chatgpt-container-bridge-extension",
    chatResponseSource: "chatgpt-container-bridge-page",
    keepAlivePort: "aistudio-container-bridge-keepalive",
  });

  globalThis.AIStudioBridgeProtocol = protocol;
  if (typeof module !== "undefined" && module.exports) module.exports = protocol;
})();


"use strict";

(() => {
  // این مخزن فقط lifecycle مربوط به provider بومی صفحه را نگهداری می‌کند.
  function createProviderStore(root = globalThis, delay = defaultDelay, config = root.AISTUDIO_UPSTREAM_CONFIG) {
    const states = [];

    function install() {
      if (!config?.attestationNamespace || !config?.attestationEntrypoint) {
        throw new Error("Attestation upstream config is missing");
      }
      const namespace = root[config.attestationNamespace] = root[config.attestationNamespace] || {};
      const entrypoint = config.attestationEntrypoint;
      const existingEntry = namespace[entrypoint];
      let runtimeEntry;

      Object.defineProperty(namespace, entrypoint, {
        configurable: true,
        enumerable: true,
        get: () => runtimeEntry,
        set: (entry) => {
          runtimeEntry = typeof entry === "function" ? wrapEntry(entry) : entry;
        },
      });

      if (existingEntry !== undefined) namespace[entrypoint] = existingEntry;
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


"use strict";

(() => {
  // این سرویس فقط snapshot تازه می‌گیرد و دربارهٔ انتخاب provider تصمیم می‌گیرد.
  function createSnapshotService(store, delay = defaultDelay, config = globalThis.AISTUDIO_UPSTREAM_CONFIG) {
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
          }, [{ [config.digestProperty]: digest }, undefined, undefined, undefined]);
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


"use strict";

(() => {
  const protocol = globalThis.AIStudioBridgeProtocol;
  const upstream = globalThis.AISTUDIO_UPSTREAM_CONFIG;
  const store = globalThis.AIStudioProviderStore.createProviderStore(globalThis, undefined, upstream);
  const snapshots = globalThis.AIStudioSnapshotService.createSnapshotService(store, undefined, upstream);
  store.install();

  // این listener درخواست content script را می‌گیرد و پاسخ را در همان origin برمی‌گرداند.
  window.addEventListener("message", async (event) => {
    if (event.source !== window || event.data?.source !== protocol.requestSource) return;
    const job = event.data;
    if (!validJob(job)) return;

    try {
      const result = await snapshots.create(job.digest, job.providerIndex);
      postResult(job, result);
    } catch (error) {
      window.postMessage({
        source: protocol.responseSource,
        jobId: job.jobId,
        error: error instanceof Error ? error.message : String(error),
      }, location.origin);
    }
  });

  function validJob(job) {
    return typeof job.jobId === "string" && /^[a-f0-9]{64}$/.test(job.digest || "");
  }

  function postResult(job, result) {
    // این داده‌ها از runtime همان صفحه خوانده می‌شوند تا client و Chrome یک هویت داشته باشند.
    const data = window[upstream.runtimeGlobal] || {};
    window.postMessage({
      source: protocol.responseSource,
      jobId: job.jobId,
      ...result,
      transportProfile: { "User-Agent": navigator.userAgent },
      runtimeConfig: {
        apiKey: typeof data[upstream.apiKeyProperty] === "string" ? data[upstream.apiKeyProperty] : undefined,
        rawVisitId: typeof data[upstream.visitIdProperty] === "string" ? data[upstream.visitIdProperty] : undefined,
        attestationEnabled: data[upstream.attestationProperty] !== false,
        authUser: String(job.authUser ?? "0"),
      },
    }, location.origin);
  }
})();

