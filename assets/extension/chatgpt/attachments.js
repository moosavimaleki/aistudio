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
