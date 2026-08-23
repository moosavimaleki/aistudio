"use strict";

(() => {
  // این نام‌ها قرارداد ثابت بین MAIN world، content script و service worker هستند.
  const protocol = Object.freeze({
    requestSource: "aistudio-container-token-bridge-extension",
    responseSource: "aistudio-container-token-bridge-page",
    jobMessage: "AISTUDIO_CONTAINER_TOKEN_JOB",
    chatJobKind: "chatgpt.generate",
    chatJobMessage: "CHATGPT_CONTAINER_CHAT_JOB",
    chatRequestSource: "chatgpt-container-bridge-extension",
    chatResponseSource: "chatgpt-container-bridge-page",
    keepAlivePort: "aistudio-container-bridge-keepalive",
  });

  globalThis.AIStudioBridgeProtocol = protocol;
  if (typeof module !== "undefined" && module.exports) module.exports = protocol;
})();


"use strict";

(() => {
  function createParser() {
    let buffer = "";
    let text = "";
    let conversationId = "";
    let upstreamError = "";

    function push(chunk) {
      buffer += chunk.replace(/\r\n/g, "\n");
      let boundary;
      while ((boundary = buffer.indexOf("\n\n")) >= 0) {
        consumeEvent(buffer.slice(0, boundary));
        buffer = buffer.slice(boundary + 2);
      }
    }

    function finish() {
      if (buffer.trim()) consumeEvent(buffer);
      return { text, conversationId, error: upstreamError };
    }

    function consumeEvent(block) {
      const data = block.split("\n")
        .filter((line) => line.startsWith("data: "))
        .map((line) => line.slice(6))
        .join("\n");
      if (!data || data === "[DONE]") return;
      try {
        consume(JSON.parse(data));
      } catch (_error) {
        // رویداد غیر JSON متعلق به stream کنترل صفحه است و نادیده گرفته می‌شود.
      }
    }

    function consume(value) {
      if (!value || typeof value !== "object") return;
      if (typeof value.conversation_id === "string") conversationId = value.conversation_id;
      applyPatch(value);
    }

    function applyPatch(patch) {
      if (patch?.p === "" && patch?.o === "patch" && Array.isArray(patch.v)) {
        patch.v.forEach(applyPatch);
        return;
      }
      if (patch?.p === "/message/content/parts/0" && patch?.o === "append" && typeof patch.v === "string") {
        text += patch.v;
        return;
      }
      if (patch?.p !== "" || patch?.o !== "add" || !patch.v || typeof patch.v !== "object") return;
      if (typeof patch.v.conversation_id === "string") conversationId = patch.v.conversation_id;
      const message = patch.v.message;
      const initial = message?.content?.parts?.[0];
      if (message?.author?.role === "assistant" && typeof initial === "string") text = initial;
      if (typeof patch.v.error?.message === "string") upstreamError = patch.v.error.message;
    }

    return { push, finish };
  }

  const api = { createParser };
  globalThis.ChatGPTSSE = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();


"use strict";

(() => {
  const protocol = globalThis.AIStudioBridgeProtocol;
  const nativeFetch = window.fetch;
  let waitingJob = "";

  window.addEventListener("message", (event) => {
    if (event.source !== window || event.origin !== location.origin) return;
    if (event.data?.source !== protocol.chatRequestSource) return;
    if (typeof event.data.jobId === "string") waitingJob = event.data.jobId;
  });

  window.fetch = async function (...args) {
    const upstreamPath = conversationPath(args[0]);
    try {
      const response = await Reflect.apply(nativeFetch, this, args);
      if (upstreamPath && waitingJob) {
        const jobId = waitingJob;
        waitingJob = "";
        observe(response.clone(), jobId, upstreamPath);
      }
      return response;
    } catch (error) {
      if (upstreamPath && waitingJob) {
        post(waitingJob, { error: message(error) });
        waitingJob = "";
      }
      throw error;
    }
  };

  function conversationPath(input) {
    try {
      const value = typeof input === "string" ? input : input?.url;
      const path = new URL(value, location.href).pathname;
      return path === "/backend-api/f/conversation" || path === "/backend-anon/f/conversation" ? path : "";
    } catch (_error) {
      return "";
    }
  }

  async function observe(response, jobId, upstreamPath) {
    if (!response.ok) {
      post(jobId, { error: `ChatGPT UI request returned HTTP ${response.status}`, upstreamStatus: response.status, upstreamPath });
      return;
    }
    if (!response.body) {
      post(jobId, { error: "ChatGPT UI response has no readable stream" });
      return;
    }
    const parser = globalThis.ChatGPTSSE.createParser();
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        parser.push(decoder.decode(value, { stream: true }));
      }
      parser.push(decoder.decode());
      const result = parser.finish();
      if (result.error) post(jobId, { error: result.error, upstreamStatus: response.status, upstreamPath });
      else post(jobId, { ...result, upstreamStatus: response.status, upstreamPath });
    } catch (error) {
      post(jobId, { error: message(error) });
    }
  }

  function post(jobId, result) {
    window.postMessage({ source: protocol.chatResponseSource, jobId, ...result }, location.origin);
  }

  function message(error) {
    return error instanceof Error ? error.message : String(error);
  }
})();

