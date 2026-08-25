"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { startKeepAlive } = require("../content/keep-alive.js");

test("keep-alive reconnects after the worker port disconnects", async () => {
  const disconnectListeners = [];
  let connections = 0;
  const runtime = {
    id: "extension-id",
    connect: () => {
      connections++;
      return {
        onDisconnect: {
          addListener: (listener) => disconnectListeners.push(listener),
        },
        postMessage: () => {},
        disconnect: () => {},
      };
    },
  };

  const stop = startKeepAlive(runtime, "keep-alive", 60_000, 0);
  disconnectListeners[0]();
  await new Promise((resolve) => setImmediate(resolve));

  assert.equal(connections, 2);
  stop();
});
