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
  function createParser() {
    let buffer = "";
    let text = "";
    let conversationId = "";
    let upstreamError = "";
    let completed = false;

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
      if (Array.isArray(patch)) {
        patch.forEach(applyPatch);
        return;
      }
      if (patch?.p === "" && Array.isArray(patch.v)) {
        patch.v.forEach(applyPatch);
        return;
      }
      if (patch?.p === "/message/content/parts/0" && patch?.o === "append" && typeof patch.v === "string") {
        text += patch.v;
        return;
      }
      if (patch?.p === "/message/status" && patch.v === "finished_successfully") completed = true;
      if (patch?.p === "/message/end_turn" && patch.v === true) completed = true;
      if (patch?.p === "/message/metadata" && patch.v?.is_complete === true) completed = true;
      if (patch?.p !== "" || !patch.v || typeof patch.v !== "object") return;
      if (typeof patch.v.conversation_id === "string") conversationId = patch.v.conversation_id;
      const message = patch.v.message;
      const initial = message?.content?.parts?.[0];
      if (message?.author?.role === "assistant" && typeof initial === "string") text = initial;
      if (typeof patch.v.error?.message === "string") upstreamError = patch.v.error.message;
      if (message?.status === "finished_successfully" || message?.end_turn === true || message?.metadata?.is_complete === true) completed = true;
    }

    return { push, finish, isComplete: () => completed };
  }

  const api = { createParser };
  globalThis.ChatGPTSSE = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();


"use strict";

(() => {
  const protocol = globalThis.AIStudioBridgeProtocol;
  const nativeFetch = window.fetch;
  let waitingJob = null;

  window.addEventListener("message", (event) => {
    if (event.source !== window || event.origin !== location.origin) return;
    if (event.data?.source !== protocol.chatRequestSource) return;
    if (typeof event.data.jobId !== "string") return;
    const job = {
      id: event.data.jobId,
      direct: event.data.direct === true,
    };
    waitingJob = job;
  });

  window.fetch = async function (...args) {
    const path = requestPath(args[0]);
    const job = waitingJob;
    if (job?.direct && path === "/backend-api/f/conversation/prepare") {
      return capturePrepare(args, job);
    }
    try {
      if (job?.direct && isConversationPath(path)) {
        waitingJob = null;
        try {
          post(job.id, await directTransport(args, path, job));
        } catch (error) {
          post(job.id, { error: message(error) });
        }
        return delegatedResponse();
      }
      const response = await Reflect.apply(nativeFetch, this, args);
      if (isConversationPath(path) && waitingJob) {
        const jobId = waitingJob.id;
        waitingJob = null;
        observe(response.clone(), jobId, path);
      }
      return response;
    } catch (error) {
      if (isConversationPath(path) && waitingJob) {
        post(waitingJob.id, { error: message(error) });
        waitingJob = null;
      }
      throw error;
    }
  };

  function requestPath(input) {
    try {
      const value = typeof input === "string" ? input : input?.url;
      return new URL(value, location.href).pathname;
    } catch (_error) {
      return "";
    }
  }

  function isConversationPath(path) {
    return path === "/backend-api/f/conversation" || path === "/backend-anon/f/conversation";
  }

  async function capturePrepare(args, job) {
    const request = new Request(args[0], args[1]);
    job.prepareHeaders = Object.fromEntries(request.headers.entries());
    return new Response(JSON.stringify({
      status: "success",
      conduit_token: `extension-delegated-${job.id}`,
    }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  }

  async function directTransport(args, upstreamPath, job) {
    const request = new Request(args[0], args[1]);
    return {
      headers: Object.fromEntries(request.headers.entries()),
      prepareHeaders: job.prepareHeaders || {},
      upstreamPath,
      context: browserContext(),
    };
  }

  function delegatedResponse() {
    const initial = {
      p: "",
      o: "add",
      v: {
        conversation_id: crypto.randomUUID(),
        message: {
          id: crypto.randomUUID(),
          author: { role: "assistant" },
          content: { content_type: "text", parts: ["Laboratory token probe completed."] },
          status: "finished_successfully",
          end_turn: true,
        },
      },
    };
    const stream = [
      'event: delta_encoding\ndata: "v1"',
      `event: delta\ndata: ${JSON.stringify(initial)}`,
      "data: [DONE]",
      "",
    ].join("\n\n");
    return new Response(stream, {
      status: 200,
      headers: { "Content-Type": "text/event-stream; charset=utf-8" },
    });
  }

  function browserContext() {
    return {
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      timezoneOffsetMin: new Date().getTimezoneOffset(),
      acceptLanguage: acceptLanguage(),
      secCHUA: clientHintBrands(),
      secCHUAMobile: navigator.userAgentData?.mobile ? "?1" : "?0",
      secCHUAPlatform: navigator.userAgentData?.platform || navigator.platform,
      isDarkMode: matchMedia("(prefers-color-scheme: dark)").matches,
      timeSinceLoaded: Math.max(1, Math.round(performance.now() / 1000)),
      pageHeight: document.documentElement.clientHeight,
      pageWidth: document.documentElement.clientWidth,
      pixelRatio: window.devicePixelRatio,
      screenHeight: window.screen.height,
      screenWidth: window.screen.width,
      hasWebPushCapabilities: "PushManager" in window,
      webPushNotificationPermission: globalThis.Notification?.permission || "default",
    };
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
        if (parser.isComplete()) {
          reader.cancel().catch(() => {});
          break;
        }
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

  function clientHintBrands() {
    const brands = navigator.userAgentData?.brands || [];
    return brands.map((item) => `"${item.brand}";v="${item.version}"`).join(", ");
  }

  function acceptLanguage() {
    return navigator.languages.map((language, index) => {
      if (index === 0) return language;
      return `${language};q=${Math.max(0.1, 1 - index / 10).toFixed(1)}`;
    }).join(",");
  }
})();

