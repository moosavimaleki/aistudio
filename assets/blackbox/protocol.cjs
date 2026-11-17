// Adapter for the external SDK contract. Property names here follow the SDK;
// the local runtime interface exposes opaque initialization and evidence data.

function registerSDKComponents(source) {
  const binding = /\b(var|let|const)\s+([A-Za-z_$][\w$]*)\s*=\s*new\s+([A-Za-z_$][\w$]*)\s*(?:\(\s*\))?\s*;/g;
  return source.replace(binding, (statement, _kind, name) =>
    `${statement}(globalThis.__blackBoxComponents ||= []).push(${name});`);
}

function getPreparationComponent(environment, digest) {
  const candidates = environment.__blackBoxComponents || [];
  const components = candidates.filter((value) => value &&
    typeof value.getRequirementsToken === "function" &&
    typeof value.getEnforcementToken === "function");
  if (components.length !== 1) {
    throw new Error(`Black Box SDK interface changed (SDK ${digest}; matches ${components.length})`);
  }
  return {
    initialize: () => components[0].getRequirementsToken(),
    evaluate: (requirements) => components[0].getEnforcementToken(requirements),
  };
}

function getSDK(environment) {
  return environment.SentinelSDK;
}

function parseEvidence(token, requirements) {
  let value;
  try { value = JSON.parse(token); }
  catch { throw new Error("Black Box SDK returned invalid evidence JSON"); }
  if (!value || typeof value !== "object" || value.e) {
    throw new Error("Black Box SDK could not generate evidence in Node");
  }
  const workToken = typeof value.p === "string" && value.p ? value.p : null;
  const runtimeToken = typeof value.t === "string" && value.t ? value.t : null;
  if (requirements?.proofofwork?.required && !workToken) {
    throw new Error("Black Box SDK returned no required work token");
  }
  if (requirements?.turnstile?.required && !runtimeToken) {
    throw new Error("Black Box SDK returned no required runtime token");
  }
  if (runtimeToken && runtimeDiagnostic(runtimeToken)) {
    throw new Error("Black Box runtime evaluation did not complete");
  }
  return { work_token: workToken, runtime_token: runtimeToken };
}

function runtimeDiagnostic(value) {
  if (/^\d+$/.test(value)) return true;
  try {
    return /^\d+: /.test(atob(value));
  } catch { return false; }
}

function needsRuntimeToken(requirements) {
  return Boolean(requirements?.turnstile?.required);
}

function diagnosticRequirements() {
  return { proofofwork: { required: true, seed: "blackbox-runtime-check", difficulty: "0fffff" } };
}

module.exports = { diagnosticRequirements, needsRuntimeToken, registerSDKComponents, getPreparationComponent, getSDK, parseEvidence };
