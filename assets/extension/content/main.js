"use strict";

(() => {
  const protocol = globalThis.AIStudioBridgeProtocol;
  const channel = globalThis.AIStudioPageChannel.createPageChannel(
    window,
    location.origin,
    protocol,
  );

  // این اتصال باعث می‌شود polling در service worker بین jobها متوقف نشود.
  globalThis.AIStudioKeepAlive.startKeepAlive(
    chrome.runtime,
    protocol.keepAlivePort,
  );

  // این listener تنها پیام job معتبر را به MAIN world صفحه عبور می‌دهد.
  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message?.type !== protocol.jobMessage) return false;
    channel.request(message, sendResponse);
    return true;
  });
})();
