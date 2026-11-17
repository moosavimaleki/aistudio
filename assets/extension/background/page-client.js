"use strict";

(() => {
  // هر job فقط به صفحه واقعی provider خودش در همان Chrome profile می‌رسد.
  function createPageClient(chromeApi, protocol, pageMatch, chatgptPageMatch) {
    async function execute(job) {
      const isChatJob = job.kind === protocol.chatJobKind ||
        job.kind === protocol.chatImageJobKind ||
        job.kind === protocol.chatDirectJobKind;
      if (isChatJob) return executeChat(job);
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
      const direct = job.kind === protocol.chatDirectJobKind;
      if (!tab?.id) {
        tab = await chromeApi.tabs.create({ url: startURL, active: false });
      } else if (!isChatPage(tab.url, startURL)) {
        tab = await chromeApi.tabs.update(tab.id, { url: startURL, active: false });
      }
      await waitForChatPage(tab.id, startURL, direct ? 90_000 : 45_000);
      const result = await sendWhenReady(tab.id, {
        type: protocol.chatJobMessage,
        jobId: job.id,
        direct: job.kind === protocol.chatDirectJobKind,
        ...(direct ? {
          submitNonce: job.submitNonce,
        } : {
          prompt: job.prompt,
          submitNonce: job.submitNonce,
          expectImage: job.kind === protocol.chatImageJobKind,
          attachments: job.attachments,
        }),
      }, direct ? 140_000 : 300_000);
      if (result?.error) throw new Error(result.error);
      if (job.kind === protocol.chatDirectJobKind) {
        if (!result?.headers || typeof result.headers !== "object") {
          throw new Error("ChatGPT page returned no direct transport headers");
        }
        return result;
      }
      if (job.kind === protocol.chatImageJobKind) {
        if (!Array.isArray(result?.images) || !result.images.length) {
          throw new Error("ChatGPT page returned no generated image");
        }
        return result;
      }
      if (typeof result?.text !== "string" || !result.text.trim()) throw new Error("ChatGPT page returned no text");
      return result;
    }

    async function findTab(match) {
      const tabs = await chromeApi.tabs.query({ url: match });
      return tabs.find((candidate) => candidate.active) || tabs[0];
    }

    async function sendWhenReady(tabId, message, responseTimeoutMs) {
      const deadline = Date.now() + 45_000;
      let lastError;
      while (Date.now() < deadline) {
        let timeoutID;
        try {
          return await Promise.race([
            chromeApi.tabs.sendMessage(tabId, message),
            new Promise((_resolve, reject) => {
              timeoutID = setTimeout(() => reject(new Error("ChatGPT page did not respond to the job")), responseTimeoutMs);
            }),
          ]);
        } catch (error) {
          lastError = error;
          if (error?.message === "ChatGPT page did not respond to the job") throw error;
          await new Promise((resolve) => setTimeout(resolve, 250));
        } finally {
          clearTimeout(timeoutID);
        }
      }
      throw lastError || new Error("ChatGPT tab did not become ready");
    }

    async function waitForChatPage(tabId, startURL, timeoutMs = 45_000) {
      const deadline = Date.now() + timeoutMs;
      let lastError;
      while (Date.now() < deadline) {
        try {
          const tab = await chromeApi.tabs.get(tabId);
          if (!isChatPage(tab.url, startURL)) {
            await new Promise((resolve) => setTimeout(resolve, 250));
            continue;
          }
          const readiness = await chromeApi.tabs.sendMessage(tabId, { type: protocol.chatReadyMessage });
          if (readiness?.ready) return;
        } catch (error) {
          lastError = error;
        }
        await new Promise((resolve) => setTimeout(resolve, 250));
      }
      throw lastError || new Error("ChatGPT tab did not finish loading");
    }

    function isChatPage(rawURL, startURL) {
      try {
        const current = new URL(rawURL);
        const start = new URL(startURL);
        return current.origin === start.origin &&
          (current.pathname === start.pathname || current.pathname.startsWith("/c/"));
      } catch (_error) {
        return false;
      }
    }

    return { execute };
  }

  const api = { createPageClient };
  globalThis.AIStudioPageClient = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
