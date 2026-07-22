"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { createFactoryClient } = require("../background/factory-client.js");

test("factory client keeps browser identity on poll and completion", async () => {
  const calls = [];
  const fetchImpl = async (url, options = {}) => {
    calls.push({ url, options });
    if (url.includes("/next")) {
      return { status: 200, ok: true, json: async () => ({ id: "job-1" }) };
    }
    return { status: 200, ok: true };
  };
  const client = createFactoryClient(fetchImpl, "http://factory:3345", "browser2");

  assert.deepEqual(await client.nextJob(), { id: "job-1" });
  await client.complete("job-1", { token: "fresh" });

  // این دو URL باید همیشه job را به extension همان Chrome profile محدود کنند.
  assert.match(calls[0].url, /browserId=browser2/);
  assert.match(calls[1].url, /browserId=browser2/);
  assert.equal(JSON.parse(calls[1].options.body).token, "fresh");
});

test("factory client converts an empty queue to null", async () => {
  const client = createFactoryClient(
    async () => ({ status: 204, ok: true }),
    "http://factory:3345",
    "default",
  );
  assert.equal(await client.nextJob(), null);
});
