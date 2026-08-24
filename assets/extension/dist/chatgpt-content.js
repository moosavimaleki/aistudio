"use strict";

(() => {
  // این نام‌ها قرارداد ثابت بین MAIN world، content script و service worker هستند.
  const protocol = Object.freeze({
    requestSource: "aistudio-container-token-bridge-extension",
    responseSource: "aistudio-container-token-bridge-page",
    jobMessage: "AISTUDIO_CONTAINER_TOKEN_JOB",
    chatJobKind: "chatgpt.generate",
    chatImageJobKind: "chatgpt.generate_image",
    chatDirectJobKind: "chatgpt.prepare_direct",
    chatJobMessage: "CHATGPT_CONTAINER_CHAT_JOB",
    chatReadyMessage: "CHATGPT_CONTAINER_READY",
    chatRequestSource: "chatgpt-container-bridge-extension",
    chatResponseSource: "chatgpt-container-bridge-page",
    keepAlivePort: "aistudio-container-bridge-keepalive",
  });

  globalThis.AIStudioBridgeProtocol = protocol;
  if (typeof module !== "undefined" && module.exports) module.exports = protocol;
})();


"use strict";

(() => {
  // این heartbeat فقط service worker را هنگام بازبودن صفحهٔ آزمایشگاه زنده نگه می‌دارد.
  function startKeepAlive(runtime, portName, intervalMs = 20_000) {
    let port;
    let timer;

    function stop() {
      if (timer) clearInterval(timer);
      timer = undefined;
      port = undefined;
    }

    try {
      port = runtime.connect({ name: portName });
      port.onDisconnect.addListener(stop);
      timer = setInterval(() => {
        try {
          if (!runtime.id || !port) return stop();
          port.postMessage({ type: "heartbeat" });
        } catch (_error) {
          stop();
        }
      }, intervalMs);
    } catch (_error) {
      stop();
    }

    return stop;
  }

  const api = { startKeepAlive };
  globalThis.AIStudioKeepAlive = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();


"use strict";

(() => {
  function createChannel(pageWindow, origin, protocol, timeoutMs = 180_000) {
    const pending = new Map();

    pageWindow.addEventListener("message", (event) => {
      if (event.source !== pageWindow || event.origin !== origin) return;
      if (event.data?.source !== protocol.chatResponseSource) return;
      const finish = pending.get(event.data.jobId);
      if (finish) finish(event.data);
    });

    function capture(jobId, options = {}) {
      let cancel;
      const result = new Promise((resolve) => {
        const timeout = setTimeout(() => {
          pending.delete(jobId);
          resolve({ error: "ChatGPT page timed out while waiting for its native response" });
        }, timeoutMs);
        cancel = () => {
          clearTimeout(timeout);
          pending.delete(jobId);
        };
        pending.set(jobId, (value) => {
          cancel();
          resolve(value);
        });
      });
      pageWindow.postMessage({ source: protocol.chatRequestSource, jobId, ...options }, origin);
      return { result, cancel };
    }

    return { capture };
  }

  const api = { createChannel };
  globalThis.ChatGPTChannel = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();


"use strict";

(() => {
  async function prepare(prompt, submitNonce, timeoutMs = 45_000) {
    await ready();
    const composer = await waitFor(() => document.querySelector("#prompt-textarea"), timeoutMs);
    composer.focus();
    selectContents(composer);
    document.execCommand("delete", false);
    if (!document.execCommand("insertText", false, prompt)) {
      composer.textContent = prompt;
    }
    composer.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertText", data: prompt }));
    composer.dataset.aistudioSubmitNonce = submitNonce;
  }

  async function submitProbe(submitNonce, timeoutMs = 45_000) {
    await prepare(".", submitNonce, timeoutMs);
    const button = await waitFor(() => {
      const candidate = document.querySelector('[data-testid="send-button"]');
      return candidate && !candidate.disabled ? candidate : null;
    }, timeoutMs, "ChatGPT probe submit button did not become ready");
    button.click();
  }

  async function ready() {
    await acceptCookieConsent();
    await dismissBlockingDialog();
    return Boolean(document.querySelector("#prompt-textarea"));
  }

  async function dismissBlockingDialog() {
    const button = [...document.querySelectorAll("button")].find((candidate) => {
      const label = candidate.textContent?.trim().toLowerCase();
      return label === "got it" || label === "close";
    });
    if (!button) return;
    button.click();
    await new Promise((resolve) => setTimeout(resolve, 250));
  }

  async function acceptCookieConsent() {
    const known = document.querySelector("#onetrust-accept-btn-handler");
    const button = known || [...document.querySelectorAll("button")].find((candidate) => {
      const label = candidate.textContent?.trim().toLowerCase();
      return label === "accept all" || label === "accept cookies" || label === "allow all";
    });
    if (!button) return;
    button.click();
    await new Promise((resolve) => setTimeout(resolve, 250));
  }

  function assistantCount() {
    return document.querySelectorAll('[data-message-author-role="assistant"]').length;
  }

  function imageSources() {
    return new Set([...document.images].map((image) => image.currentSrc).filter(Boolean));
  }

  async function readGeneratedImages(previousSources, timeoutMs = 90_000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const source = [...document.images]
        .find((image) => image.naturalWidth >= 64 && !previousSources.has(image.currentSrc))?.currentSrc;
      if (source) return [await readImage(source)];
      await delay(250);
    }
    return [];
  }

  async function readImage(source) {
    const response = await fetch(source, { credentials: "include" });
    if (!response.ok) throw new Error(`ChatGPT image download returned HTTP ${response.status}`);
    const blob = await response.blob();
    if (!blob.type.startsWith("image/")) throw new Error("ChatGPT image download returned a non-image response");
    const dataURL = await new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result);
      reader.onerror = () => reject(reader.error || new Error("ChatGPT image could not be encoded"));
      reader.readAsDataURL(blob);
    });
    const separator = String(dataURL).indexOf(",");
    return { mimeType: blob.type, data: String(dataURL).slice(separator + 1) };
  }

  async function readLastAssistant(afterCount, timeoutMs = 30_000) {
    const deadline = Date.now() + timeoutMs;
    let stableText = "";
    let stableSince = 0;
    while (Date.now() < deadline) {
      const messages = [...document.querySelectorAll('[data-message-author-role="assistant"]')];
      if (messages.length <= afterCount) {
        await delay(100);
        continue;
      }
      const last = messages.at(-1);
      const content = last?.querySelector(".markdown") || last;
      const text = content?.innerText?.trim() || "";
      if (text !== stableText) {
        stableText = text;
        stableSince = Date.now();
      }
      if (text && !isGenerating() && Date.now() - stableSince >= 1_000) return text;
      await delay(100);
    }
    throw new Error("ChatGPT final assistant message did not become ready");
  }

  function isGenerating() {
    return Boolean(document.querySelector(
      '[data-testid="stop-button"], [data-testid="stop-generating-button"], button[aria-label*="Stop"]',
    ));
  }

  function selectContents(element) {
    const selection = window.getSelection();
    const range = document.createRange();
    range.selectNodeContents(element);
    selection.removeAllRanges();
    selection.addRange(range);
  }

  async function waitFor(find, timeoutMs, errorMessage = "ChatGPT composer is unavailable; sign in or inspect the page through noVNC") {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const value = find();
      if (value) return value;
      await delay(100);
    }
    throw new Error(errorMessage);
  }

  const delay = (milliseconds) => new Promise((resolve) => setTimeout(resolve, milliseconds));

  const api = { ready, prepare, submitProbe, assistantCount, imageSources, readGeneratedImages, readLastAssistant };
  globalThis.ChatGPTComposer = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();


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

