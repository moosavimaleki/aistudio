"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");

const { createParser } = require("../chatgpt/sse.js");

test("ChatGPT SSE parser joins only assistant content patches", () => {
  const parser = createParser();
  parser.push('event: delta\ndata: {"p":"","o":"add","v":{"conversation_id":"c1","message":{"author":{"role":"assistant"},"content":{"parts":[""]}}}}\n\n');
  parser.push('event: delta\ndata: {"p":"","o":"patch","v":[{"p":"/message/content/parts/0","o":"append","v":"hel"},{"p":"/message/metadata/model_slug","o":"replace","v":"ignored"}]}\n\n');
  parser.push('event: delta\ndata: {"p":"/message/content/parts/0","o":"append","v":"lo"}\n\n');

  assert.deepEqual(parser.finish(), { text: "hello", conversationId: "c1", error: "" });
});

test("ChatGPT SSE parser exposes upstream error without protected fields", () => {
  const parser = createParser();
  parser.push('event: delta\ndata: {"p":"","o":"add","v":{"error":{"message":"request failed"}}}\n\n');

  assert.equal(parser.finish().error, "request failed");
});
