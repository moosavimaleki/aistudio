"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

test("manifest references only existing extension modules", () => {
  const root = path.resolve(__dirname, "..");
  const manifest = JSON.parse(fs.readFileSync(path.join(root, "manifest.json"), "utf8"));
  const scripts = [
    manifest.background.service_worker,
    ...manifest.content_scripts.flatMap((entry) => entry.js),
  ];

  // این تست مانع build شدن افزونه با path شکسته بعد از refactor می‌شود.
  for (const script of scripts) {
    assert.equal(fs.existsSync(path.join(root, script)), true, script);
  }
});
