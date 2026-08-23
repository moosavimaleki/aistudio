"use strict";

(() => {
  // هر job فقط به صفحه واقعی provider خودش در همان Chrome profile می‌رسد.
  function createPageClient(chromeApi, protocol, pageMatch, chatgptPageMatch) {
    async function execute(job) {
      if (job.kind === protocol.chatJobKind) return executeChat(job);
      if (!pageMatch) throw new Error("AI Studio page match is missing from extension config");
      const tab = await findTab(pageMatch);
      if (!tab?.id) throw new Error("Container Chrome has no AI Studio tab");

      const result = await chromeApi.tabs.sendMessage(tab.id, {
        type: protocol.jobMessage,
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

    async function executeChat(job) {
      if (!chatgptPageMatch) throw new Error("ChatGPT page match is missing from extension config");
      let tab = await findTab(chatgptPageMatch);
      const startURL = chatgptPageMatch.replace(/\*.*$/, "");
      if (!tab?.id) {
        tab = await chromeApi.tabs.create({ url: startURL, active: false });
      }
      const result = await sendWhenReady(tab.id, {
        type: protocol.chatJobMessage,
        jobId: job.id,
        prompt: job.prompt,
        submitNonce: job.submitNonce,
      });
      if (result?.error) throw new Error(result.error);
      if (typeof result?.text !== "string" || !result.text.trim()) throw new Error("ChatGPT page returned no text");
      return result;
    }

    async function findTab(match) {
      const tabs = await chromeApi.tabs.query({ url: match });
      return tabs.find((candidate) => candidate.active) || tabs[0];
    }

    async function sendWhenReady(tabId, message) {
      const deadline = Date.now() + 45_000;
      let lastError;
      while (Date.now() < deadline) {
        try {
          return await chromeApi.tabs.sendMessage(tabId, message);
        } catch (error) {
          lastError = error;
          await new Promise((resolve) => setTimeout(resolve, 250));
        }
      }
      throw lastError || new Error("ChatGPT tab did not become ready");
    }

    return { execute };
  }

  const api = { createPageClient };
  globalThis.AIStudioPageClient = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
