"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const { findMatch } = require("../shared/image-match.js");

function image(width, height, color = [240, 240, 240]) {
  const data = new Uint8ClampedArray(width * height * 4);
  for (let offset = 0; offset < data.length; offset += 4) {
    data.set([...color, 255], offset);
  }
  return { width, height, data };
}

function paint(imageData, x, y, color) {
  const offset = (y * imageData.width + x) * 4;
  imageData.data.set([...color, 255], offset);
}

test("finds a visible image and tolerates small color changes", () => {
  const reference = image(16, 12);
  const screen = image(80, 60);
  for (let y = 0; y < reference.height; y++) {
    for (let x = 0; x < reference.width; x++) {
      const color = [(x * 17) % 256, (y * 21) % 256, ((x + y) * 11) % 256];
      paint(reference, x, y, color);
      paint(screen, x + 24, y + 18, color.map((part) => Math.min(255, part + 7)));
    }
  }
  const match = findMatch(screen, reference);
  assert.ok(match);
  assert.equal(match.x, 24);
  assert.equal(match.y, 18);
  assert.ok(match.score > 0.95);
});

test("does not accept a missing or unrelated image", () => {
  const reference = image(16, 12, [20, 40, 60]);
  const screen = image(80, 60, [240, 240, 240]);
  assert.equal(findMatch(screen, reference), null);
  assert.equal(findMatch(screen, null), null);
});
