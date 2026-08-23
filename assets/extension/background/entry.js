"use strict";

// این ترتیب import dependencyهای کوچک worker را پیش از شروع polling آماده می‌کند.
importScripts(
  "../config/runtime-config.js",
  "../shared/protocol.js",
  "config.js",
  "factory-client.js",
  "page-client.js",
  "worker.js",
);

const config = globalThis.AIStudioWorkerConfig;
const protocol = globalThis.AIStudioBridgeProtocol;
const factory = globalThis.AIStudioFactoryClient.createFactoryClient(
  fetch,
  config.factoryOrigin,
  config.browserId,
);
const page = globalThis.AIStudioPageClient.createPageClient(
  chrome,
  protocol,
  config.pageMatch,
  config.chatgptPageMatch,
);

globalThis.AIStudioWorker.startWorker({ chromeApi: chrome, factory, page, protocol });
