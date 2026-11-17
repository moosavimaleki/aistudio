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
    document.documentElement.dataset.aistudioJobStage = `received:${message.jobId}`;
    run(message).then((result) => {
      document.documentElement.dataset.aistudioJobStage = `finished:${message.jobId}`;
      sendResponse(result);
    });
    return true;
  });

  async function run(job) {
    const direct = job.direct === true;
    if (!direct && (typeof job.prompt !== "string" || !job.prompt.trim())) {
      return { error: "ChatGPT prompt is empty" };
    }
    const previousAssistantCount = globalThis.ChatGPTComposer.assistantCount();
    const previousImages = globalThis.ChatGPTComposer.imageSources();
    const previousRateLimits = job.expectImage ? globalThis.ChatGPTComposer.imageRateLimits() : new Set();
    const capture = channel.capture(job.jobId, {
      direct,
      expectImage: job.expectImage === true,
    });
    try {
      await capture.ready;
      document.documentElement.dataset.aistudioJobStage = `armed:${job.jobId}`;
      if (direct) {
        await globalThis.ChatGPTComposer.submitProbe(job.submitNonce || job.jobId);
        document.documentElement.dataset.aistudioJobStage = `prepared:${job.jobId}`;
        // Reuse the mounted page/composer. The next probe replaces its input;
        // Go owns the actual conversation and does not need a fresh UI chat.
        return await capture.result;
      }
		await globalThis.ChatGPTComposer.prepare(job.prompt, job.submitNonce || job.jobId, 45_000, job.attachments);
      const result = await capture.result;
      if (result.error) return result;
      if (job.expectImage) {
        const images = await globalThis.ChatGPTComposer.readGeneratedImages(previousImages, previousRateLimits);
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
