"use strict";

(() => {
  const intervalMs = 30_000;
  const threshold = 0.86;
  let busy = false;
  let alreadyClicked = false;

  async function imageData(source) {
    const bitmap = await createImageBitmap(source);
    try {
      const canvas = document.createElement("canvas");
      canvas.width = bitmap.width;
      canvas.height = bitmap.height;
      const context = canvas.getContext("2d", { willReadFrequently: true });
      context.drawImage(bitmap, 0, 0);
      return context.getImageData(0, 0, canvas.width, canvas.height);
    } finally {
      bitmap.close();
    }
  }

  async function check() {
    if (busy || document.visibilityState !== "visible") return;
    busy = true;
    try {
      const reference = await fetch(chrome.runtime.getURL("click.png"), { cache: "no-store" });
      if (!reference.ok) return; // No click.png: the optional feature stays inactive.
      const captured = await chrome.runtime.sendMessage({ type: "aistudio.captureVisibleTab" });
      if (!captured?.image) return;
      const [needle, screenshot] = await Promise.all([
        imageData(await reference.blob()),
        imageData(await (await fetch(captured.image)).blob()),
      ]);
      const match = globalThis.AIStudioImageMatch.findMatch(screenshot, needle, threshold);
      if (!match) {
        alreadyClicked = false;
        return;
      }
      if (alreadyClicked || document.visibilityState !== "visible") return;
      const x = (match.x + match.width / 2) * innerWidth / screenshot.width;
      const y = (match.y + match.height / 2) * innerHeight / screenshot.height;
      const target = document.elementFromPoint(x, y);
      if (!target || target.tagName === "IFRAME") return;
      target.click();
      alreadyClicked = true;
    } catch (_error) {
      // A missing image, unavailable capture, or navigation is not a page error.
    } finally {
      busy = false;
    }
  }

  setInterval(check, intervalMs);
})();
