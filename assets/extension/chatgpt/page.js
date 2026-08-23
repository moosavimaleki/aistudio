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
