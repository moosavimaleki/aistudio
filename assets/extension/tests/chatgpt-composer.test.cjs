"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { imageRateLimits, imageRateLimitReached, isGeneratedImage, ready, submitProbe, resetAfterProbe } = require("../chatgpt/composer.js");

test("composer is ready with the current contenteditable textbox", async () => {
  const originalDocument = global.document;
  global.document = {
    querySelector: () => null,
    querySelectorAll: (selector) => selector.includes("[data-composer-markdown]") ? [{}] : [],
  };
  try {
    assert.equal(await ready(), true);
  } finally {
    global.document = originalDocument;
  }
});

test("direct probe prepares the composer for a trusted Chrome submit", async () => {
  const originalDocument = global.document;
  const originalWindow = global.window;
  const originalInputEvent = global.InputEvent;
  let clicked = false;
  const composer = { dataset: {}, focus() {}, dispatchEvent() {} };
  const button = {
    disabled: false,
    getAttribute: (name) => name === "aria-label" ? "Send" : null,
    click: () => { clicked = true; },
  };
  global.document = {
    documentElement: { dataset: {} },
    querySelector: (selector) => selector.includes("[data-composer-markdown]") ? composer : null,
    querySelectorAll: (selector) => selector.includes("[data-composer-markdown]") ? [composer] : [button],
    createRange: () => ({ selectNodeContents() {} }),
    execCommand: () => true,
  };
  global.window = { getSelection: () => ({ removeAllRanges() {}, addRange() {} }) };
  global.InputEvent = class {};
  try {
    await submitProbe("probe-1", 100);
    assert.equal(composer.dataset.aistudioSubmitNonce, "probe-1");
    assert.equal(global.document.documentElement.dataset.aistudioSubmitNonce, "probe-1");
    assert.equal(clicked, false);
  } finally {
    global.document = originalDocument;
    global.window = originalWindow;
    global.InputEvent = originalInputEvent;
  }
});

test("direct probe ignores a hidden duplicate composer", async () => {
  const originalDocument = global.document;
  const originalWindow = global.window;
  const originalInputEvent = global.InputEvent;
  const hidden = { dataset: {}, getClientRects: () => [], focus() { throw new Error("hidden composer focused"); } };
  const visible = { dataset: {}, getClientRects: () => [{}], focus() {}, dispatchEvent() {} };
  global.document = {
    documentElement: { dataset: {} },
    querySelector: (selector) => selector.includes("[data-composer-markdown]") ? hidden : null,
    querySelectorAll: (selector) => selector.includes("[data-composer-markdown]") ? [hidden, visible] : [],
    createRange: () => ({ selectNodeContents() {} }),
    execCommand: () => true,
  };
  global.window = { getSelection: () => ({ removeAllRanges() {}, addRange() {} }) };
  global.InputEvent = class {};
  try {
    await submitProbe("probe-visible", 100);
    assert.equal(visible.dataset.aistudioSubmitNonce, "probe-visible");
    assert.equal(hidden.dataset.aistudioSubmitNonce, undefined);
  } finally {
    global.document = originalDocument;
    global.window = originalWindow;
    global.InputEvent = originalInputEvent;
  }
});

test("direct probe returns the browser to a clean chat without reloading", async () => {
  const originalDocument = global.document;
  const originalLocation = global.location;
  let clicked = false;
  global.location = { pathname: "/c/probe-only" };
  global.document = {
    querySelector: () => ({}),
    querySelectorAll: (selector) => selector.includes('data-message-author-role') ? [] : selector.includes("[data-composer-markdown]") ? [{}] : [{
      getAttribute: (name) => name === "aria-label" ? "New chat" : null,
      click: () => { clicked = true; global.location.pathname = "/"; },
    }],
  };
  try {
    await resetAfterProbe("probe-only", 100);
    assert.equal(clicked, true);
    assert.equal(global.location.pathname, "/");
  } finally {
    global.document = originalDocument;
    global.location = originalLocation;
  }
});

test("probe cleanup skips hidden New chat buttons", async () => {
  const originalDocument = global.document;
  const originalLocation = global.location;
  let visibleClicked = false;
  global.location = { pathname: "/c/probe-only" };
  const hidden = {
    getAttribute: () => "New chat",
    getClientRects: () => [],
    click() { throw new Error("hidden New chat button clicked"); },
  };
  const visible = {
    getAttribute: () => "New chat",
    getClientRects: () => [{}],
    click() { visibleClicked = true; global.location.pathname = "/"; },
  };
  global.document = {
    querySelectorAll: (selector) => selector.includes('data-message-author-role') ? [] : selector.includes("[data-composer-markdown]") ? [{}] : [hidden, visible],
  };
  try {
    await resetAfterProbe("probe-only", 100);
    assert.equal(visibleClicked, true);
    assert.equal(global.location.pathname, "/");
  } finally {
    global.document = originalDocument;
    global.location = originalLocation;
  }
});

test("probe cleanup accepts a missing ID and a local conversation route", async () => {
  const previousDocument = global.document;
  const previousLocation = global.location;
  global.location = { pathname: "/c/local-chatgpt%3Aprobe" };
  const composer = { textContent: "" };
  const link = { getAttribute: (key) => key === "data-testid" ? "create-new-chat-button" : null,
    click: () => { global.location.pathname = "/"; } };
  global.document = { querySelectorAll: (selector) => selector.includes("data-message-author-role") ? [] :
    selector.includes("data-composer-markdown") ? [composer] : [link] };
  try { await resetAfterProbe(undefined, 100); assert.equal(location.pathname, "/"); }
  finally { global.document = previousDocument; global.location = previousLocation; }
});

test("generated image selector rejects UI avatars", () => {
  const previous = new Set();
  const avatar = {
    currentSrc: "https://cdn.auth0.com/avatars/user.png",
    naturalWidth: 120,
    naturalHeight: 120,
    alt: "Profile image",
  };
  assert.equal(isGeneratedImage(avatar, previous), false);
});

test("generated image selector accepts the ChatGPT output asset", () => {
  const source = "https://chatgpt.com/backend-api/estuary/content?id=test";
  const image = { currentSrc: source, naturalWidth: 1254, naturalHeight: 1254, alt: "Generated image: Blue circle" };
  assert.equal(isGeneratedImage(image, new Set()), true);
  assert.equal(isGeneratedImage(image, new Set([source])), false);
});

test("image quota detection ignores an old card but catches a new one", () => {
  const originalDocument = global.document;
  const oldCard = {};
  const newCard = {};
  let cards = [oldCard];
  global.document = {
    querySelectorAll: () => cards,
  };
  try {
    const previous = imageRateLimits();
    assert.equal(imageRateLimitReached(previous), false);
    cards = [oldCard, newCard];
    assert.equal(imageRateLimitReached(previous), true);
  } finally {
    global.document = originalDocument;
  }
});
