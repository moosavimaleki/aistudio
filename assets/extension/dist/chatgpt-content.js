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
    chatCaptureReadySource: "chatgpt-container-bridge-ready",
    chatResponseSource: "chatgpt-container-bridge-page",
    keepAlivePort: "aistudio-container-bridge-keepalive",
  });

  globalThis.AIStudioBridgeProtocol = protocol;
  if (typeof module !== "undefined" && module.exports) module.exports = protocol;
})();


"use strict";

(() => {
  // این heartbeat فقط service worker را هنگام بازبودن صفحهٔ آزمایشگاه زنده نگه می‌دارد.
  function startKeepAlive(runtime, portName, intervalMs = 20_000, retryMs = 1_000) {
    let port;
    let timer;
    let retryTimer;
    let stopped = false;

    function clearConnection() {
      if (timer) clearInterval(timer);
      timer = undefined;
      port = undefined;
    }

    function reconnect() {
      clearConnection();
      if (stopped || retryTimer) return;
      retryTimer = setTimeout(() => {
        retryTimer = undefined;
        connect();
      }, retryMs);
    }

    function connect() {
      if (stopped || !runtime.id) return;
      try {
        const connection = runtime.connect({ name: portName });
        port = connection;
        connection.onDisconnect.addListener(() => {
          if (port === connection) reconnect();
        });
        timer = setInterval(() => {
          try {
            connection.postMessage({ type: "heartbeat" });
          } catch (_error) {
            reconnect();
          }
        }, intervalMs);
      } catch (_error) {
        reconnect();
      }
    }

    function stop() {
      stopped = true;
      clearConnection();
      if (retryTimer) clearTimeout(retryTimer);
      retryTimer = undefined;
    }

    connect();
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
      const capture = pending.get(event.data?.jobId);
      if (!capture) return;
      if (event.data.source === protocol.chatCaptureReadySource) {
        capture.arm();
        return;
      }
      if (event.data.source === protocol.chatResponseSource) capture.finish(event.data);
    });

    function capture(jobId, options = {}) {
      let arm;
      let finish;
      let resultTimeout;
      let readyTimeout;
      const ready = new Promise((resolve, reject) => {
        readyTimeout = setTimeout(() => {
          pending.delete(jobId);
          reject(new Error("ChatGPT page hook did not become ready"));
        }, Math.min(timeoutMs, 10_000));
        arm = () => {
          clearTimeout(readyTimeout);
          resolve();
        };
      });
      const result = new Promise((resolve) => {
        resultTimeout = setTimeout(() => {
          pending.delete(jobId);
          resolve({ error: "ChatGPT page timed out while waiting for its native response" });
        }, timeoutMs);
        finish = (value) => {
          arm();
          clearTimeout(resultTimeout);
          pending.delete(jobId);
          resolve(value);
        };
      });
      const cancel = () => {
        arm();
        clearTimeout(resultTimeout);
        pending.delete(jobId);
      };
      pending.set(jobId, { arm, finish });
      pageWindow.postMessage({ source: protocol.chatRequestSource, jobId, ...options }, origin);
      return { ready, result, cancel };
    }

    return { capture };
  }

  const api = { createChannel };
  globalThis.ChatGPTChannel = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();


"use strict";

(() => {
  const composerSelector = "#prompt-textarea, [data-composer-markdown][contenteditable=true]";

  async function prepare(prompt, submitNonce, timeoutMs = 45_000, attachments = []) {
    await ready();
    const composer = await waitFor(findComposer, timeoutMs);
    composer.focus();
    selectContents(composer);
    document.execCommand("delete", false);
    if (!document.execCommand("insertText", false, prompt)) {
      composer.textContent = prompt;
    }
    composer.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertText", data: prompt }));
    if (attachments?.length) {
      await waitFor(findSubmitButton, timeoutMs, "ChatGPT composer did not accept the image prompt");
      await delay(250);
      await globalThis.ChatGPTAttachments.attach(attachments, timeoutMs);
    }
    composer.dataset.aistudioSubmitNonce = submitNonce;
    document.documentElement.dataset.aistudioSubmitNonce = submitNonce;
  }

  async function submitProbe(submitNonce, timeoutMs = 45_000) {
    await prepare(".", submitNonce, timeoutMs);
  }

  async function resetAfterProbe(probeConversationId, timeoutMs = 30_000) {
    // React can retain a local conversation URL or deliver captured artifacts
    // before its route settles. Cleanup depends on an empty composer, not on
    // a fabricated probe ID becoming the final URL.
    const clean = () => location.pathname === "/" && findComposer() &&
      !(findComposer().textContent || "").trim() &&
      document.querySelectorAll('[data-message-author-role="user"]').length === 0;
    if (clean()) return;
    const newChat = await waitFor(() => [...document.querySelectorAll("button, a")].find(
      (button) => isVisible(button) && (button.getAttribute("aria-label") === "New chat" ||
        button.getAttribute("data-testid") === "create-new-chat-button" ||
        (button.getAttribute("href") === "/" && /new chat/i.test(button.textContent || ""))),
    ), timeoutMs, "ChatGPT New chat button is unavailable after the probe");
    if (!newChat) throw new Error("ChatGPT New chat button is unavailable after the probe");
    newChat.click();
    await waitFor(clean, timeoutMs,
      "ChatGPT did not return to a clean chat after the probe");
  }

  function findComposer() {
    return [...document.querySelectorAll(composerSelector)].find(isVisible) || null;
  }

  function isVisible(element) {
    return typeof element.getClientRects !== "function" || element.getClientRects().length > 0;
  }

  function findSubmitButton() {
    return [...document.querySelectorAll("button")].find((candidate) => {
      if (candidate.disabled || !isVisible(candidate)) return false;
      const testID = candidate.getAttribute("data-testid") || "";
      const label = (candidate.getAttribute("aria-label") || "").trim().toLowerCase();
      return testID === "send-button" || label === "send" || label.startsWith("send ");
    }) || null;
  }

  async function ready() {
    await acceptCookieConsent();
    await dismissBlockingDialog();
    return Boolean(findComposer());
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

  function imageRateLimits() {
    return new Set(document.querySelectorAll('[data-testid="image-gen-rate-limit-tombstone"]'));
  }

  function imageRateLimitReached(previousRateLimits) {
    return [...imageRateLimits()].some((card) => !previousRateLimits.has(card));
  }

  async function readGeneratedImages(previousSources, previousRateLimits = new Set(), timeoutMs = 240_000) {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const source = [...document.images]
        .find((image) => isGeneratedImage(image, previousSources))?.currentSrc;
      if (source) return [await readImage(source)];
      if (imageRateLimitReached(previousRateLimits)) {
        throw new Error("ChatGPT image generation limit reached for this account");
      }
      await delay(250);
    }
    return [];
  }

  function isGeneratedImage(image, previousSources) {
    const source = image.currentSrc || image.src || "";
    if (!source || previousSources.has(source)) return false;
    if (image.naturalWidth < 256 || image.naturalHeight < 256) return false;
    if (image.alt?.trim().toLowerCase().startsWith("generated image")) return true;
    try {
      const url = new URL(source, globalThis.location?.href || "https://chatgpt.com/");
      return url.origin === "https://chatgpt.com" && url.pathname === "/backend-api/estuary/content";
    } catch (_error) {
      return false;
    }
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

  const api = { ready, prepare, submitProbe, resetAfterProbe, assistantCount, imageSources, imageRateLimits, imageRateLimitReached, isGeneratedImage, readGeneratedImages, readLastAssistant };
  globalThis.ChatGPTComposer = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();


"use strict";

(() => {
  async function attach(attachments, timeoutMs = 45_000) {
    const input = document.querySelector('input[type="file"][aria-label="Attach photos"]') ||
      document.querySelector('input[type="file"][accept*="image/"]');
    if (!input) throw new Error("ChatGPT image upload input is unavailable");
    const transfer = new DataTransfer();
    for (const attachment of attachments) {
      if (!attachment.data || !attachment.mimeType?.startsWith("image/")) {
        throw new Error("Invalid reference image attachment");
      }
      const binary = atob(attachment.data);
      const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
      transfer.items.add(new File([bytes], attachment.name, { type: attachment.mimeType }));
    }
    input.files = transfer.files;
    input.dispatchEvent(new Event("change", { bubbles: true }));

    const deadline = Date.now() + timeoutMs;
    let readySince = 0;
    while (Date.now() < deadline) {
      const form = input.closest("form") || input.parentElement;
      const panel = form?.querySelector('[data-composer-attachments]');
      const previews = [...(panel?.querySelectorAll("img[alt]") || [])];
      const pending = panel?.querySelector('[role="progressbar"], [aria-label*="Uploading"]');
      const uploaded = previews.length >= attachments.length;
      if (uploaded && !pending) {
        if (!readySince) readySince = Date.now();
        if (Date.now() - readySince >= 1_000) return;
      } else {
        readySince = 0;
      }
      await new Promise((resolve) => setTimeout(resolve, 200));
    }
    const panel = document.querySelector('[data-composer-attachments]');
    const count = panel?.querySelectorAll("img[alt]").length || 0;
    throw new Error(`ChatGPT reference images did not finish uploading (${count}/${attachments.length} previews)`);
  }

  const api = { attach };
  globalThis.ChatGPTAttachments = api;
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

