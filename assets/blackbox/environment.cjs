// Browser API adapter for SDK evaluation without a rendering engine.

function createEnvironment({ implementation, requirements: suppliedRequirements, initial }) {
  const requestedElements = [];
  const appendedElements = [];
  const listeners = new Map();
  const noop = () => {};
  let environment;
  const bridgeWindow = {
    postMessage: (message) => {
      if (!message?.requestId || !listeners.has("message")) return;
      const requirements = suppliedRequirements || {
        token: "fixture-requirements-token",
        proofofwork: {
          required: true,
          seed: "blackbox-runtime-check",
          difficulty: "0fffff",
        },
        turnstile: { required: true, dx: "fixture-runtime-config" },
      };
      setTimeout(() => listeners.get("message")({
        source: bridgeWindow,
        origin: "https://chatgpt.com",
        data: {
          type: "response",
          requestId: message.requestId,
          result: {
            cachedChatReq: requirements,
            cachedProof: initial || message.p,
          },
        },
      }), 0);
    },
  };
  const currentScript = { src: implementation };
  const document = {
    cookie: "",
    currentScript,
    scripts: [currentScript],
    location: { href: implementation, origin: "https://chatgpt.com" },
    documentElement: { getAttribute: () => null },
    body: {
      appendChild: (element) => { appendedElements.push(element.tagName); return element; },
      removeChild: (element) => element,
    },
    createElement: (tag) => {
      requestedElements.push(tag);
      return {
        tagName: tag.toUpperCase(),
        // No graphics context is available in this non-rendering environment.
        ...(tag.toLowerCase() === "canvas" ? { getContext: () => null } : {}),
        style: {},
        // The runtime has no renderer: its elements have no layout box.
        getBoundingClientRect: () => ({ x: 0, y: 0, top: 0, right: 0,
          bottom: 0, left: 0, width: 0, height: 0 }),
        remove: noop,
        setAttribute: noop,
        addEventListener: (name, callback) => {
          if (name === "load") setTimeout(callback, 0);
        },
        removeEventListener: noop,
        contentWindow: bridgeWindow,
      };
    },
    addEventListener: noop,
    removeEventListener: noop,
    querySelector: () => null,
  };
  environment = {
    console: { log: noop, info: noop, warn: noop, error: noop, debug: noop },
    crypto: globalThis.crypto,
    TextEncoder,
    TextDecoder,
    URL,
    URLSearchParams,
    setTimeout,
    clearTimeout,
    setInterval,
    clearInterval,
    performance,
    Response,
    Request,
    Headers,
    AbortController,
    atob,
    btoa,
    screen: { width: 1920, height: 1080, colorDepth: 24, pixelDepth: 24 },
    navigator: {
      userAgent: `Node.js ${process.version}`,
      language: "en-US",
      languages: ["en-US"],
      platform: process.platform,
      hardwareConcurrency: 8,
      deviceMemory: 8,
      plugins: [],
      mimeTypes: [],
      maxTouchPoints: 0,
      vendor: "",
    },
    document,
    history: { length: 1, state: null },
    localStorage: createMemoryStorage(),
    sessionStorage: createMemoryStorage(),
    location: { href: "https://chatgpt.com/", origin: "https://chatgpt.com", search: "", ancestorOrigins: [] },
    requestIdleCallback: (callback) => setTimeout(
      () => callback({ timeRemaining: () => 10, didTimeout: false }), 0),
    addEventListener: (name, callback) => listeners.set(name, callback),
    removeEventListener: (name) => listeners.delete(name),
    dispatchEvent: noop,
    fetch: async () => { throw new Error("SDK network access is handled by the client"); },
  };
  environment.window = environment;
  environment.self = environment;
  environment.top = environment;
  environment.parent = environment;
  return { environment, requestedElements, appendedElements, listeners };
}

function createMemoryStorage() {
  const values = new Map();
  return {
    get length() { return values.size; },
    getItem: (key) => values.get(String(key)) ?? null,
    setItem: (key, value) => { values.set(String(key), String(value)); },
    removeItem: (key) => { values.delete(String(key)); },
    clear: () => values.clear(),
    key: (index) => [...values.keys()][index] ?? null,
  };
}

module.exports = { createEnvironment, createMemoryStorage };
