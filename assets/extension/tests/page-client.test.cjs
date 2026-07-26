"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { createPageClient } = require("../background/page-client.js");

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
  const client = createPageClient(chromeApi, "TOKEN_JOB", "https://aistudio.google.com/*");

  const result = await client.execute({ id: "j", digest: "a".repeat(64), authUser: "0" });

  // این تست تضمین می‌کند job به tab فعال همان process ارسال می‌شود.
  assert.equal(sent[0].tabId, 20);
  assert.equal(sent[0].message.type, "TOKEN_JOB");
  assert.equal(result.token, "fresh");
});
