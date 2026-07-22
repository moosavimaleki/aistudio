"use strict";

(() => {
  // این client job را فقط به tab واقعی AI Studio در همان Chrome profile می‌رساند.
  function createPageClient(chromeApi, jobMessage) {
    async function execute(job) {
      const tabs = await chromeApi.tabs.query({ url: "https://aistudio.google.com/*" });
      const tab = tabs.find((candidate) => candidate.active) || tabs[0];
      if (!tab?.id) throw new Error("Container Chrome has no AI Studio tab");

      const result = await chromeApi.tabs.sendMessage(tab.id, {
        type: jobMessage,
        jobId: job.id,
        digest: job.digest,
        authUser: job.authUser,
        providerIndex: job.providerIndex,
      });
      if (result?.error) throw new Error(result.error);
      if (typeof result?.token !== "string" || !result.token) {
        throw new Error("AI Studio page returned no token");
      }
      return result;
    }

    return { execute };
  }

  const api = { createPageClient };
  globalThis.AIStudioPageClient = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
