"use strict";

(() => {
  async function prepare(prompt, submitNonce, timeoutMs = 45_000) {
    await acceptCookieConsent();
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

  async function readLastAssistant(timeoutMs = 60_000) {
    const node = await waitFor(() => {
      const messages = [...document.querySelectorAll('[data-message-author-role="assistant"]')];
      const last = messages.at(-1);
      const content = last?.querySelector(".markdown") || last;
      return content?.innerText?.trim() ? content : null;
    }, timeoutMs);
    return node.innerText.trim();
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
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    throw new Error(errorMessage);
  }

  const api = { prepare, readLastAssistant };
  globalThis.ChatGPTComposer = api;
  if (typeof module !== "undefined" && module.exports) module.exports = api;
})();
