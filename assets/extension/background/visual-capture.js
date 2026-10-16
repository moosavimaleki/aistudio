"use strict";

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (message?.type !== "aistudio.captureVisibleTab" || !sender.tab?.id || sender.frameId !== 0) return false;
  (async () => {
    const tab = await chrome.tabs.get(sender.tab.id);
    if (!tab.active || tab.windowId !== sender.tab.windowId) return {};
    const matches = [
      globalThis.AIStudioWorkerConfig.pageMatch,
      globalThis.AIStudioWorkerConfig.chatgptPageMatch,
    ];
    if (!matches.some((pattern) => pattern && tab.url?.startsWith(pattern.split("*")[0]))) return {};
    const image = await chrome.tabs.captureVisibleTab(tab.windowId, { format: "png" });
    return { image };
  })().then(sendResponse, () => sendResponse({}));
  return true;
});
