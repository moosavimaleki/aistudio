"use strict";

(() => {
  // این client فقط قرارداد HTTP داخلی extension با browser-interface را پیاده می‌کند.
  function createFactoryClient(fetchImpl, factoryOrigin, browserId) {
    const query = `browserId=${encodeURIComponent(browserId)}`;

    async function nextJob() {
      const response = await fetchImpl(
        `${factoryOrigin}/internal/jobs/next?${query}`,
        { cache: "no-store" },
      );
      if (response.status === 204) return null;
      if (!response.ok) throw new Error(`Job polling failed with HTTP ${response.status}`);
      return response.json();
    }

    async function complete(jobId, result) {
      const response = await fetchImpl(
        `${factoryOrigin}/internal/jobs/${encodeURIComponent(jobId)}/result?${query}`,
        {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify(result),
        },
      );
      if (!response.ok && response.status !== 404) {
        throw new Error(`Result delivery failed with HTTP ${response.status}`);
      }
    }

    return { nextJob, complete };
  }

  const api = { createFactoryClient };
  globalThis.AIStudioFactoryClient = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
