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
      create: async (options) => ({ id: 31, ...options }),
      sendMessage: async (tabId, message) => {
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
