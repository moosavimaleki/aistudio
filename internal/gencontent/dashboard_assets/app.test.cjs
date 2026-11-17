const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

async function renderFixture(mode) {
  const elements = new Map();
  const element = selector => {
    if (!elements.has(selector)) elements.set(selector, {
      innerHTML: "", textContent: "", hidden: false,
      classList: { toggle() {} }, addEventListener() {},
    });
    return elements.get(selector);
  };
  const data = {
    chatMode: mode, generatedAt: new Date().toISOString(),
    direct: { state: "CONFIGURED", credentialType: "cookies", credentialFiles: 4 },
    summary: { requests: 3, success: 3, errors: 0, successRate: 100, latencyP50: 1000, latencyP95: 2500 },
    series: [{ timestamp: Date.now(), requests: 3, success: 3, errors: 0 }],
    timings: [{ name: "black_box_evaluate", count: 3, averageMs: 1500, p50Ms: 2500, p95Ms: 2500 }],
    latencyBuckets: [{ upperMs: 1000, count: 2 }, { upperMs: 2500, count: 1 }, { upperMs: null, count: 0 }],
    errorPhases: {}, errorStatuses: {}, profiles: [], models: [], events: [],
    pool: mode === "direct" ? {} : { total: 0, max: 3, available: 0, leased: 0 },
  };
  const context = vm.createContext({
    document: { querySelector: element, querySelectorAll: () => [], body: { classList: { toggle() {} } } },
    fetch: async url => {
      assert.match(url, /^\/dashboard\/data\?/);
      return { ok: true, json: async () => data };
    },
    setInterval: () => 0,
  });
  vm.runInContext(fs.readFileSync(path.join(__dirname, "app.js"), "utf8"), context);
  await vm.runInContext("refresh()", context);
  return element;
}

test("direct view hides browser state and renders real metric fields", async () => {
  const element = await renderFixture("direct");
  assert.equal(element("#browser-sessions").hidden, true);
  assert.equal(element("#browser-pool").hidden, true);
  assert.equal(element("#direct-session").hidden, false);
  assert.match(element("#summary").innerHTML, /direct/);
  assert.doesNotMatch(element("#summary").innerHTML, /undefined/);
  assert.match(element("#latency-bars").innerHTML, /≤ 1s/);
  assert.match(element("#timings").innerHTML, /1.5s/);
  assert.match(element("#request-chart").innerHTML, /<circle/);
});

test("browser view retains browser sessions and pool", async () => {
  const element = await renderFixture("browser");
  assert.equal(element("#browser-sessions").hidden, false);
  assert.equal(element("#browser-pool").hidden, false);
  assert.equal(element("#direct-session").hidden, true);
});
