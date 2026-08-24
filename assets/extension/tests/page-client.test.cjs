"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { createPageClient } = require("../background/page-client.js");
const protocol = require("../shared/protocol.js");

test("page client forwards one job to the active AI Studio tab", async () => {
  const sent = [];
  const chromeApi = {
    tabs: {
      query: async () => [{ id: 10 }, { id: 20, active: true }],
      get: async () => ({ id: 20, status: "complete", url: "https://chatgpt.com/" }),
      sendMessage: async (tabId, message) => {
        sent.push({ tabId, message });
        return { token: "fresh", providerCount: 1 };
      },
    },
  };
  const client = createPageClient(chromeApi, protocol, "https://aistudio.google.com/*", "https://chatgpt.com/*");

  const result = await client.execute({ id: "j", digest: "a".repeat(64), authUser: "0" });

  // این تست تضمین می‌کند job به tab فعال همان process ارسال می‌شود.
  assert.equal(sent[0].tabId, 20);
  assert.equal(sent[0].message.type, protocol.jobMessage);
  assert.equal(result.token, "fresh");
});

test("page client reloads ChatGPT and forwards only prompt data", async () => {
  const sent = [];
  const chromeApi = {
    tabs: {
      query: async () => [{ id: 30, active: false }],
      update: async (id, update) => ({ id, ...update }),
      get: async () => ({ id: 30, status: "complete", url: "https://chatgpt.com/" }),
      create: async (options) => ({ id: 31, ...options }),
      sendMessage: async (tabId, message) => {
        if (message.type === protocol.chatReadyMessage) return { ready: true };
        sent.push({ tabId, message });
        return { text: "answer", conversationId: "conversation", upstreamStatus: 200 };
      },
    },
  };
  const client = createPageClient(chromeApi, protocol, "https://aistudio.google.com/*", "https://chatgpt.com/*");

  const result = await client.execute({ id: "chat-1", kind: protocol.chatJobKind, prompt: "hello", submitNonce: "submit-1" });

  assert.equal(sent[0].message.type, protocol.chatJobMessage);
  assert.equal(sent[0].message.prompt, "hello");
  assert.equal(sent[0].message.submitNonce, "submit-1");
  assert.equal(Object.hasOwn(sent[0].message, "headers"), false);
  assert.equal(result.text, "answer");
});

test("page client marks a ChatGPT image job without adding protected request data", async () => {
  const sent = [];
  const chromeApi = {
    tabs: {
      query: async () => [{ id: 40, active: true }],
      update: async (id, update) => ({ id, ...update }),
      get: async () => ({ id: 40, status: "complete", url: "https://chatgpt.com/" }),
      sendMessage: async (tabId, message) => {
        if (message.type === protocol.chatReadyMessage) return { ready: true };
        sent.push({ tabId, message });
        return { images: [{ mimeType: "image/png", data: "aW1hZ2U=" }] };
      },
    },
  };
  const client = createPageClient(chromeApi, protocol, "https://aistudio.google.com/*", "https://chatgpt.com/*");

  const result = await client.execute({ id: "image-1", kind: protocol.chatImageJobKind, prompt: "draw", submitNonce: "submit-2" });

  assert.equal(sent[0].message.expectImage, true);
  assert.equal(Object.hasOwn(sent[0].message, "headers"), false);
  assert.equal(result.images[0].mimeType, "image/png");
});

test("page client requests browser artifacts for a direct Go turn", async () => {
  const sent = [];
  let updateCalls = 0;
  const chromeApi = {
    tabs: {
      query: async () => [{ id: 50, active: true }],
      update: async (id, update) => {
        updateCalls++;
        return { id, ...update };
      },
      get: async () => ({ id: 50, status: "complete", url: "https://chatgpt.com/c/existing" }),
      sendMessage: async (tabId, message) => {
        if (message.type === protocol.chatReadyMessage) return { ready: true };
        sent.push({ tabId, message });
        return {
          headers: { "openai-sentinel-proof-token": "opaque" },
          prepareHeaders: { "x-openai-target-path": "/backend-api/f/conversation/prepare" },
        };
      },
    },
  };
  const client = createPageClient(chromeApi, protocol, "https://aistudio.google.com/*", "https://chatgpt.com/*");

  await client.execute({
    id: "direct-1",
    kind: protocol.chatDirectJobKind,
    prompt: "hello",
    model: "gpt-5-6-pro",
    conversationId: "conversation-1",
    parentMessageId: "message-1",
    thinkingEffort: "standard",
  });

  assert.equal(sent[0].message.direct, true);
  assert.equal(sent[0].message.model, "gpt-5-6-pro");
  assert.equal(sent[0].message.conversationId, "conversation-1");
  assert.equal(sent[0].message.parentMessageId, "message-1");
  assert.equal(updateCalls, 0);
});
