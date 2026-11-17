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
