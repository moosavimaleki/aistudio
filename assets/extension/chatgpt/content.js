"use strict";

(() => {
  const protocol = globalThis.AIStudioBridgeProtocol;
  const channel = globalThis.ChatGPTChannel.createChannel(window, location.origin, protocol);
	globalThis.AIStudioKeepAlive.startKeepAlive(chrome.runtime, protocol.keepAlivePort);

  chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message?.type === protocol.chatReadyMessage) {
      Promise.resolve(globalThis.ChatGPTComposer?.ready?.())
        .then((ready) => sendResponse({ ready: Boolean(ready) }))
        .catch(() => sendResponse({ ready: false }));
      return true;
    }
    if (message?.type !== protocol.chatJobMessage) return false;
    run(message).then(sendResponse);
    return true;
  });

  async function run(job) {
    const direct = job.direct === true;
    if (!direct && (typeof job.prompt !== "string" || !job.prompt.trim())) {
      return { error: "ChatGPT prompt is empty" };
    }
    const previousAssistantCount = globalThis.ChatGPTComposer.assistantCount();
    const previousImages = globalThis.ChatGPTComposer.imageSources();
    const capture = channel.capture(job.jobId, {
      direct,
    });
    try {
      await capture.ready;
      if (direct) {
        await globalThis.ChatGPTComposer.submitProbe(job.submitNonce || job.jobId);
        const result = await capture.result;
        setTimeout(() => location.reload(), 500);
        return result;
      }
      await globalThis.ChatGPTComposer.prepare(job.prompt, job.submitNonce || job.jobId);
      const result = await capture.result;
      if (result.error) return result;
      if (job.expectImage) {
        const images = await globalThis.ChatGPTComposer.readGeneratedImages(previousImages);
        if (!images.length) return { error: "ChatGPT page returned no generated image" };
        return { ...result, images };
      }
      try {
        return { ...result, text: await globalThis.ChatGPTComposer.readLastAssistant(previousAssistantCount) };
      } catch (_error) {
        return result;
      }
    } catch (error) {
      capture.cancel();
      return { error: error instanceof Error ? error.message : String(error) };
    }
  }
})();
