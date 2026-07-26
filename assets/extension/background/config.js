"use strict";

(() => {
  // این تنظیمات از نسخهٔ مخصوص همان Chrome profile خوانده می‌شوند.
  const source = globalThis.AISTUDIO_BRIDGE_CONFIG || {};
  globalThis.AIStudioWorkerConfig = Object.freeze({
    browserId: source.browserId || "default",
    factoryOrigin: source.factoryOrigin || "http://127.0.0.1:3345",
    pageMatch: source.pageMatch,
  });
})();
