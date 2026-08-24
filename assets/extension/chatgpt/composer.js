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
