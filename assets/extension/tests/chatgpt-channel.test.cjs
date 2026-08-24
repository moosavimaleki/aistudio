"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { createChannel } = require("../chatgpt/channel.js");
const protocol = require("../shared/protocol.js");

class FakeWindow {
  constructor() {
    this.listeners = [];
  }

  addEventListener(type, listener) {
    if (type === "message") this.listeners.push(listener);
  }

  postMessage() {}

  emit(origin, data) {
    for (const listener of this.listeners) {
      listener({ source: this, origin, data });
    }
  }
}

test("ChatGPT capture waits until the page hook is armed", async () => {
  const origin = "https://chatgpt.com";
  const pageWindow = new FakeWindow();
  const capture = createChannel(pageWindow, origin, protocol, 1_000).capture("job-1", {
    direct: true,
  });

  let armed = false;
  capture.ready.then(() => { armed = true; });
  await Promise.resolve();
  assert.equal(armed, false);

  pageWindow.emit(origin, {
    source: protocol.chatCaptureReadySource,
    jobId: "job-1",
  });

  await capture.ready;
  assert.equal(armed, true);

  pageWindow.emit(origin, {
    source: protocol.chatResponseSource,
    jobId: "job-1",
    headers: { "openai-sentinel-proof-token": "opaque" },
  });
  assert.deepEqual((await capture.result).headers, {
    "openai-sentinel-proof-token": "opaque",
  });
});
