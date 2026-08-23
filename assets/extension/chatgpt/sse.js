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
