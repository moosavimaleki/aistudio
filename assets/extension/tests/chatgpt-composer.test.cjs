"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { isGeneratedImage } = require("../chatgpt/composer.js");

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
