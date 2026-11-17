#!/usr/bin/env node

// Black Box runtime for client-side SDK calculations. Authentication and
// upstream HTTP requests remain in the Go client.
const runtimeContext = require("node:vm");
const { loadSdk } = require("./sdk_cache.cjs");
const { createEnvironment } = require("./environment.cjs");
const { registerSDKComponents, getPreparationComponent, getSDK, parseEvidence,
  needsRuntimeToken, diagnosticRequirements } = require("./protocol.cjs");

function readStdinJson() {
  return new Promise((resolve, reject) => {
    let input = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => { input += chunk; });
    process.stdin.on("end", () => {
      try { resolve(JSON.parse(input)); }
      catch (_error) { reject(new Error("invalid requirements JSON on stdin")); }
    });
  });
}

async function main() {
  const supplied = process.argv.includes("--requirements-stdin")
    ? await readStdinJson() : null;
  const suppliedRequirements = supplied?.requirements || supplied;
  const suppliedInitial = supplied?.initial;
  const loadedSdk = await loadSdk();
  const implementation = loadedSdk.implementation;
  let source = loadedSdk.source;
  // Find candidates structurally, then validate their runtime interface. Minified
  // binding names change between releases; the runtime methods identify the internal interface.
  source = registerSDKComponents(source);

  const { environment, requestedElements, appendedElements, listeners } = createEnvironment({
    implementation, requirements: suppliedRequirements, initial: suppliedInitial,
  });
  runtimeContext.createContext(environment);
  runtimeContext.runInContext(source, environment, { filename: "blackbox-sdk.js", timeout: 5000 });

  const component = getPreparationComponent(environment, loadedSdk.sha256);
  const sdk = getSDK(environment);

  if (process.argv.includes("--emit-initial")) {
    const initial = await component.initialize();
    process.stdout.write(`${JSON.stringify({ initial })}\n`);
    return;
  }

  const requirements = suppliedRequirements || diagnosticRequirements();
  if (process.argv.includes("--emit-evidence-json")) {
    if (needsRuntimeToken(requirements) &&
        (typeof suppliedInitial !== "string" || !suppliedInitial)) {
      throw new Error("Black Box evaluation requires the original initialization context");
    }
    // Bind the server requirements to the original initialization context.
    const evidence = parseEvidence(
      await sdk.token("__default__"), requirements);
    process.stdout.write(`${JSON.stringify(evidence)}\n`);
    return;
  }
  const workToken = await component.evaluate(requirements);
  let tokenResult;
  let tokenError;
  try {
    tokenResult = await Promise.race([
      sdk.token("blackbox-runtime-check"),
      new Promise((_, reject) => setTimeout(
        () => reject(new Error("token generation timed out")), 3000)),
    ]);
  } catch (error) {
    tokenError = error.message;
  }
  let tokenKeys = [];
  let tokenShape = {};
  if (typeof tokenResult === "string") {
    try {
      const parsedToken = JSON.parse(tokenResult);
      tokenKeys = Object.keys(parsedToken).sort();
      tokenShape = Object.fromEntries(tokenKeys.map((key) => {
        const value = parsedToken[key];
        return [key, {
          type: value === null ? "null" : typeof value,
          length: typeof value === "string" ? value.length : null,
        }];
      }));
    }
    catch (_error) { tokenKeys = []; }
  }
  const result = {
    sdk_url: implementation,
    sdk_sha256: loadedSdk.sha256,
    cache_status: loadedSdk.cacheStatus,
    public_api: Object.keys(sdk || {}).sort(),
    work_token_generated: typeof workToken === "string" && workToken.startsWith("gAAAAAB"),
    work_token_length: typeof workToken === "string" ? workToken.length : 0,
    requested_elements: [...new Set(requestedElements)],
    appended_elements: [...new Set(appendedElements)],
    message_bridge_installed: listeners.has("message"),
    // This bridge uses synthetic requirements; it is not a server acceptance test.
    full_token_uses_synthetic_requirements: !suppliedRequirements,
    full_token_generated: typeof tokenResult === "string",
    full_token_keys: tokenKeys,
    full_token_shape: tokenShape,
    full_token_error: tokenError ? "token generation failed" : null,
  };
  process.stdout.write(`${JSON.stringify(result)}\n`);
}

if (require.main === module) main().catch(() => {
  process.stderr.write("blackbox-runtime: execution failed\n");
  process.exitCode = 1;
});
