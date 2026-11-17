const test = require("node:test");
const assert = require("node:assert/strict");
const { parseEvidence, getPreparationComponent } = require("./protocol.cjs");
const { createEnvironment } = require("./environment.cjs");

test("SDK evidence maps to the local runtime contract", () => {
  const result = parseEvidence('{"p":"fixture-work","t":"fixture-runtime"}', {
    proofofwork: { required: true }, turnstile: { required: true },
  });
  assert.deepEqual(result, { work_token: "fixture-work", runtime_token: "fixture-runtime" });
});

test("incomplete or diagnostic SDK output is rejected without exposing input", () => {
  for (const value of ["fixture-private-token", '{"e":"fixture-private-token"}', "null"]) {
    assert.throws(() => parseEvidence(value, {}), error =>
      !error.message.includes("fixture-private-token"));
  }
  assert.throws(() => parseEvidence("{}", { proofofwork: { required: true } }), /work token/);
  assert.throws(() => parseEvidence("{}", { turnstile: { required: true } }), /runtime token/);
  for (const value of ["123", btoa("123: diagnostic")]) {
    assert.throws(() => parseEvidence(JSON.stringify({ t: value }), {}), /did not complete/);
  }
});

test("the SDK adapter preserves its external method contract", async () => {
  const component = getPreparationComponent({ __blackBoxComponents: [{
    getRequirementsToken: async () => "fixture-initial",
    getEnforcementToken: async requirements => requirements.value,
  }] }, "fixture-digest");
  assert.equal(await component.initialize(), "fixture-initial");
  assert.equal(await component.evaluate({ value: "fixture-work" }), "fixture-work");
  assert.throws(() => getPreparationComponent({}, "fixture-digest"), /interface changed/);
});

test("the environment binds requirements to the original initialization", async () => {
  const requirements = { token: "fixture-requirements" };
  const { environment, listeners } = createEnvironment({
    implementation: "https://example.test/sdk.js", requirements, initial: "fixture-initial",
  });
  const frame = environment.document.createElement("iframe");
  const event = await new Promise(resolve => {
    listeners.set("message", resolve);
    frame.contentWindow.postMessage({ requestId: "fixture-request", p: "different-context" });
  });
  assert.equal(event.data.result.cachedChatReq, requirements);
  assert.equal(event.data.result.cachedProof, "fixture-initial");
  assert.equal(environment.document.cookie, "");
  await assert.rejects(environment.fetch("https://example.test"), /handled by the client/);
});
