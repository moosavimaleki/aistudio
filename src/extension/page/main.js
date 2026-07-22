"use strict";

(() => {
  const protocol = globalThis.AIStudioBridgeProtocol;
  const store = globalThis.AIStudioProviderStore.createProviderStore(globalThis);
  const snapshots = globalThis.AIStudioSnapshotService.createSnapshotService(store);
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
    const data = window.WIZ_global_data || {};
    window.postMessage({
      source: protocol.responseSource,
      jobId: job.jobId,
      ...result,
      transportProfile: { "User-Agent": navigator.userAgent },
      runtimeConfig: {
        apiKey: typeof data.WIu0Nc === "string" ? data.WIu0Nc : undefined,
        rawVisitId: typeof data.teM9xe === "string" ? data.teM9xe : undefined,
        authUser: String(job.authUser ?? "0"),
      },
    }, location.origin);
  }
})();
