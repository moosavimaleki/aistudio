"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

test("captures only the active supported top-level tab", async () => {
  let listener;
  let captures = 0;
  globalThis.AIStudioWorkerConfig = {
    pageMatch: "https://aistudio.google.com/*",
    chatgptPageMatch: "https://chatgpt.com/*",
  };
  globalThis.chrome = {
    runtime: { onMessage: { addListener(callback) { listener = callback; } } },
    tabs: {
      async get() { return { active: true, windowId: 3, url: "https://chatgpt.com/" }; },
      async captureVisibleTab() { captures++; return "data:image/png;base64,test"; },
    },
  };
  require("../background/visual-capture.js");
  const request = (sender) => new Promise((resolve) => {
    const asyncResponse = listener({ type: "aistudio.captureVisibleTab" }, sender, resolve);
    assert.equal(asyncResponse, true);
  });
  assert.deepEqual(await request({ tab: { id: 1, windowId: 3 }, frameId: 0 }), {
    image: "data:image/png;base64,test",
  });
  assert.deepEqual(await request({ tab: { id: 1, windowId: 4 }, frameId: 0 }), {});
  assert.equal(captures, 1);
  assert.equal(listener({ type: "aistudio.captureVisibleTab" }, { tab: { id: 1 }, frameId: 1 }, () => {}), false);
});
