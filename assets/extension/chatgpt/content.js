"use strict";

(() => {
  const protocol = globalThis.AIStudioBridgeProtocol;
  const channel = globalThis.ChatGPTChannel.createChannel(window, location.origin, protocol);
	globalThis.AIStudioKeepAlive.startKeepAlive(chrome.runtime, protocol.keepAlivePort);

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message?.type !== protocol.chatJobMessage) return false;
    run(message).then(sendResponse);
    return true;
  });

  async function run(job) {
    if (typeof job.prompt !== "string" || !job.prompt.trim()) return { error: "ChatGPT prompt is empty" };
    const capture = channel.capture(job.jobId);
    try {
      await globalThis.ChatGPTComposer.prepare(job.prompt, job.submitNonce || job.jobId);
      const result = await capture.result;
      if (result.error || result.text?.trim()) return result;
      return { ...result, text: await globalThis.ChatGPTComposer.readLastAssistant() };
    } catch (error) {
      capture.cancel();
      return { error: error instanceof Error ? error.message : String(error) };
    }
  }
})();
