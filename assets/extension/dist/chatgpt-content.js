"use strict";

(() => {
  // این نام‌ها قرارداد ثابت بین MAIN world، content script و service worker هستند.
  const protocol = Object.freeze({
    requestSource: "aistudio-container-token-bridge-extension",
    responseSource: "aistudio-container-token-bridge-page",
    jobMessage: "AISTUDIO_CONTAINER_TOKEN_JOB",
    chatJobKind: "chatgpt.generate",
    chatJobMessage: "CHATGPT_CONTAINER_CHAT_JOB",
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

    function capture(jobId) {
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
      pageWindow.postMessage({ source: protocol.chatRequestSource, jobId }, origin);
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

