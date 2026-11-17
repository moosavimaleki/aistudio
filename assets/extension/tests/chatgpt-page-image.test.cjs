"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const protocol = require("../shared/protocol.js");

test("image capture does not read the conversation response stream", async () => {
  const messages = [];
  let onMessage;
  const pageWindow = {
    addEventListener: (_type, listener) => { onMessage = listener; },
    postMessage: (message) => { messages.push(message); },
    fetch: async () => ({
      ok: true,
      status: 200,
      clone: () => { throw new Error("BodyStreamBuffer was aborted"); },
    }),
  };
  const context = {
    window: pageWindow,
    URL,
    location: { origin: "https://chatgpt.com", href: "https://chatgpt.com/" },
    AIStudioBridgeProtocol: protocol,
  };
  vm.runInNewContext(fs.readFileSync(require.resolve("../chatgpt/page.js"), "utf8"), context);
  onMessage({ source: pageWindow, origin: context.location.origin, data: {
    source: protocol.chatRequestSource,
    jobId: "image-1",
    expectImage: true,
  } });

  await pageWindow.fetch("https://chatgpt.com/backend-api/f/conversation");
  assert.equal(messages.at(-1).upstreamStatus, 200);
  assert.equal(messages.at(-1).error, undefined);
});

test("direct probe reports the same disposable conversation ID that its UI response uses", async () => {
  const messages = [];
  let onMessage;
  const pageWindow = {
    addEventListener: (_type, listener) => { onMessage = listener; },
    postMessage: (message) => { messages.push(message); },
    fetch: async () => new Response("unrelated", { status: 200 }),
    screen: { height: 900, width: 1440 },
    devicePixelRatio: 1,
  };
  const context = {
    window: pageWindow,
    URL, Request, Response, Intl,
    crypto: { randomUUID: () => "probe-123" },
    location: { origin: "https://chatgpt.com", href: "https://chatgpt.com/" },
    document: { documentElement: { clientHeight: 800, clientWidth: 1200 } },
    navigator: { languages: ["en-US"], platform: "Linux" },
    performance: { now: () => 1000 },
    matchMedia: () => ({ matches: false }),
    AIStudioBridgeProtocol: protocol,
  };
  vm.runInNewContext(fs.readFileSync(require.resolve("../chatgpt/page.js"), "utf8"), context);
  onMessage({ source: pageWindow, origin: context.location.origin, data: {
    source: protocol.chatRequestSource,
    jobId: "direct-1",
    direct: true,
  } });

  const unrelated = await pageWindow.fetch("https://chatgpt.com/backend-api/f/conversation", {
    method: "POST",
    body: JSON.stringify({ messages: [{ author: { role: "user" }, content: { parts: ["not the probe"] } }] }),
  });
  assert.equal(await unrelated.text(), "unrelated");
  assert.equal(messages.some((message) => message.probeConversationId), false);

  const response = await pageWindow.fetch("https://chatgpt.com/backend-api/f/conversation", {
    method: "POST",
    body: JSON.stringify({ messages: [{ author: { role: "user" }, content: { parts: ["."] } }] }),
  });
  const stream = await response.text();

  assert.equal(messages.at(-1).probeConversationId, "probe-123");
  assert.match(stream, /"conversation_id":"probe-123"/);
});
