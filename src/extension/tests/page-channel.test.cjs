"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const protocol = require("../shared/protocol.js");
const { createPageChannel } = require("../content/page-channel.js");

test("page channel returns only the response for its pending job", async () => {
  let listener;
  let posted;
  const pageWindow = {
    addEventListener: (_type, callback) => { listener = callback; },
    postMessage: (message) => { posted = message; },
  };
  const channel = createPageChannel(pageWindow, "https://aistudio.google.com", protocol, 1_000);
  const response = new Promise((resolve) => {
    channel.request({ jobId: "job-1", digest: "a".repeat(64) }, resolve);
  });

  listener({
    source: pageWindow,
    origin: "https://aistudio.google.com",
    data: { source: protocol.responseSource, jobId: "job-1", token: "fresh" },
  });

  // این assertion جداسازی responseهای jobهای هم‌زمان را کنترل می‌کند.
  assert.equal(posted.source, protocol.requestSource);
  assert.equal((await response).token, "fresh");
});
