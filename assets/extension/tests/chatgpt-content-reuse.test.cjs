"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const protocol = require("../shared/protocol.js");

test("successive direct jobs reuse the page without New chat or navigation", async () => {
  let listener;
  const probes = [];
  const captures = [];
  const location = Object.freeze({ origin: "https://chatgpt.com", pathname: "/c/existing" });
  const context = {
    window: {}, location,
    document: { documentElement: { dataset: {} } },
    chrome: { runtime: { onMessage: { addListener: (fn) => { listener = fn; } } } },
    AIStudioBridgeProtocol: protocol,
    AIStudioKeepAlive: { startKeepAlive() {} },
    ChatGPTChannel: { createChannel: () => ({ capture(id) {
      captures.push(id);
      return { ready: Promise.resolve(), result: Promise.resolve({ headers: { fixture: id } }),
        cancel() { throw new Error("successful probe was cancelled"); } };
    } }) },
    ChatGPTComposer: {
      assistantCount: () => 0, imageSources: () => new Set(),
      submitProbe: async (nonce) => { probes.push(nonce); },
      resetAfterProbe() { throw new Error("New chat must not run between direct jobs"); },
    },
  };
  vm.runInNewContext(fs.readFileSync(require.resolve("../chatgpt/content.js"), "utf8"), context);
  for (const id of ["turn-1", "turn-2", "turn-3"]) {
    const result = await new Promise((resolve) => listener({
      type: protocol.chatJobMessage, direct: true, jobId: id, submitNonce: id,
    }, null, resolve));
    assert.equal(result.headers.fixture, id);
    assert.equal(result.cleanupRequired, undefined);
    assert.equal(location.pathname, "/c/existing");
  }
  assert.deepEqual(probes, ["turn-1", "turn-2", "turn-3"]);
  assert.deepEqual(captures, probes);
});

test("a failed direct capture remains an error without navigation recovery", async () => {
  let listener;
  const context = {
    window: {}, location: { origin: "https://chatgpt.com" },
    document: { documentElement: { dataset: {} } },
    chrome: { runtime: { onMessage: { addListener: (fn) => { listener = fn; } } } },
    AIStudioBridgeProtocol: protocol,
    AIStudioKeepAlive: { startKeepAlive() {} },
    ChatGPTChannel: { createChannel: () => ({ capture: () => ({
      ready: Promise.resolve(), result: Promise.resolve({ error: "fixture unavailable" }),
    }) }) },
    ChatGPTComposer: { assistantCount: () => 0, imageSources: () => new Set(), submitProbe: async () => {} },
  };
  vm.runInNewContext(fs.readFileSync(require.resolve("../chatgpt/content.js"), "utf8"), context);
  const result = await new Promise((resolve) => listener({
    type: protocol.chatJobMessage, direct: true, jobId: "failed-turn",
  }, null, resolve));
  assert.equal(result.error, "fixture unavailable");
});
