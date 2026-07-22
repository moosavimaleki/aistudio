"use strict";

(() => {
  // این نام‌ها قرارداد ثابت بین MAIN world، content script و service worker هستند.
  const protocol = Object.freeze({
    requestSource: "aistudio-container-token-bridge-extension",
    responseSource: "aistudio-container-token-bridge-page",
    jobMessage: "AISTUDIO_CONTAINER_TOKEN_JOB",
    keepAlivePort: "aistudio-container-bridge-keepalive",
  });

  globalThis.AIStudioBridgeProtocol = protocol;
  if (typeof module !== "undefined" && module.exports) module.exports = protocol;
})();
