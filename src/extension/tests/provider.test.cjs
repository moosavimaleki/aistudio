"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { createProviderStore } = require("../page/provider-store.js");
const { createSnapshotService } = require("../page/snapshot-service.js");

test("provider hook captures native snapshots and primes only once", async () => {
  const root = { botguard: {} };
  const config = {
    attestationNamespace: "botguard",
    attestationEntrypoint: "a",
    digestProperty: "content",
  };
  const store = createProviderStore(root, async () => {}, config);
  store.install();

  let snapshotCount = 0;
  root.botguard.a = (_program, onReady) => {
    onReady((callback) => callback(`token-${++snapshotCount}`), () => {});
  };
  root.botguard.a(null, () => {});

  const service = createSnapshotService(store, async () => {}, config);
  const first = await service.create("a".repeat(64));
  const second = await service.create("b".repeat(64));

  // این assertion ثابت می‌کند snapshot آماده‌سازی هرگز به‌عنوان token نهایی برنمی‌گردد.
  assert.equal(first.token, "token-2");
  assert.equal(second.token, "token-3");
  assert.equal(first.providerCount, 1);
});
