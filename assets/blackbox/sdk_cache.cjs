// Loads and caches the SDK used by the Black Box runtime.
const cryptoModule = require("crypto");
const fs = require("fs");
const os = require("os");
const path = require("path");

const bootstrapUrl = "https://chatgpt.com/backend-api/sentinel/sdk.js";
const cacheTtlMs = Number(
  process.env.CHATGPT_BLACK_BOX_CACHE_TTL_MS || "21600000");

function sha256(value) {
  return cryptoModule.createHash("sha256").update(value).digest("hex");
}

function cacheDirectory() {
  const root = process.env.XDG_CACHE_HOME || path.join(os.homedir(), ".cache");
  return path.join(root, "aistudio-api", "blackbox");
}

function readCachedSdk() {
  const directory = cacheDirectory();
  const manifestPath = path.join(directory, "current.json");
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  const sourcePath = path.join(directory, manifest.file);
  const source = fs.readFileSync(sourcePath, "utf8");
  if (sha256(source) !== manifest.sha256) {
    throw new Error("cached Black Box SDK failed its SHA-256 check");
  }
  return { ...manifest, source, manifestPath };
}

function writeAtomic(file, value) {
  const temporary = `${file}.tmp-${process.pid}`;
  fs.writeFileSync(temporary, value, { mode: 0o600, flag: "wx" });
  fs.renameSync(temporary, file);
  fs.chmodSync(file, 0o600);
}

async function loadSdk() {
  let cached = null;
  try { cached = readCachedSdk(); }
  catch (_error) { cached = null; }
  const refresh = process.argv.includes("--refresh");
  if (!refresh && cached && Date.now() - cached.checked_at < cacheTtlMs) {
    return { ...cached, cacheStatus: "hit" };
  }
  try {
    const bootstrapResponse = await fetch(bootstrapUrl, { signal: AbortSignal.timeout(10000) });
    if (!bootstrapResponse.ok) throw new Error(`bootstrap HTTP ${bootstrapResponse.status}`);
    const bootstrap = await bootstrapResponse.text();
    const implementation = bootstrap.match(/script\.src\s*=\s*["']([^"']+)["']/)?.[1];
    if (!implementation) throw new Error("Black Box SDK URL not found");
    const sourceResponse = await fetch(implementation, { signal: AbortSignal.timeout(10000) });
    if (!sourceResponse.ok) throw new Error(`SDK HTTP ${sourceResponse.status}`);
    const source = await sourceResponse.text();
    const digest = sha256(source);
    const directory = cacheDirectory();
    fs.mkdirSync(directory, { recursive: true, mode: 0o700 });
    fs.chmodSync(directory, 0o700);
    const file = `sdk-${digest.slice(0, 16)}.js`;
    const sourcePath = path.join(directory, file);
    if (!fs.existsSync(sourcePath)) writeAtomic(sourcePath, source);
    const manifest = { version: 1, implementation, file, sha256: digest,
      checked_at: Date.now() };
    writeAtomic(path.join(directory, "current.json"), `${JSON.stringify(manifest)}\n`);
    return { ...manifest, source, cacheStatus: cached ? "updated" : "miss" };
  } catch (error) {
    if (cached && !refresh) return { ...cached, cacheStatus: "stale-offline" };
    throw error;
  }
}


module.exports = { loadSdk };
