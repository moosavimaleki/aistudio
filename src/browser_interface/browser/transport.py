"""Capture the network identity of the container Chrome."""

from typing import Any

TRANSPORT_SCRIPT = """async () => {
  const data = navigator.userAgentData;
  const high = typeof data?.getHighEntropyValues === "function"
    ? await data.getHighEntropyValues([
        "architecture", "bitness", "formFactors", "fullVersionList",
        "model", "platformVersion", "wow64"
      ])
    : {};
  return {
    userAgent: navigator.userAgent,
    brands: data?.brands,
    mobile: data?.mobile,
    platform: data?.platform,
    ...high,
  };
}"""

RPC_IDENTITY_HEADERS = (
    "accept-language", "user-agent", "sec-ch-ua", "sec-ch-ua-arch",
    "sec-ch-ua-bitness", "sec-ch-ua-form-factors", "sec-ch-ua-full-version",
    "sec-ch-ua-full-version-list", "sec-ch-ua-mobile", "sec-ch-ua-model",
    "sec-ch-ua-platform", "sec-ch-ua-platform-version", "sec-ch-ua-wow64",
    "x-browser-channel", "x-browser-copyright", "x-browser-validation",
    "x-browser-year", "x-client-data",
)


async def read_transport_profile(page: Any) -> dict[str, str]:
    browser = await page.evaluate(TRANSPORT_SCRIPT)
    profile = {
        "Accept": "*/*",
        "Accept-Language": "en-US,en;q=0.9,fa;q=0.8,cs;q=0.7",
        "User-Agent": browser["userAgent"],
        "Priority": "u=1, i",
        "sec-fetch-dest": "empty",
        "sec-fetch-mode": "cors",
        "sec-fetch-site": "same-site",
    }
    _add_client_hints(profile, browser)
    return profile


def rpc_transport_profile(headers: dict[str, Any]) -> dict[str, str]:
    source = {str(name).lower(): value for name, value in headers.items()}
    return {
        name: str(source[name])
        for name in RPC_IDENTITY_HEADERS
        if isinstance(source.get(name), str) and source[name]
    }


def normalize_headers(headers: dict[str, Any]) -> dict[str, str]:
    return {str(name).lower(): str(value) for name, value in headers.items()}


def _add_client_hints(profile: dict[str, str], browser: dict[str, Any]) -> None:
    brands = _format_brands(browser.get("brands"))
    full_versions = _format_brands(browser.get("fullVersionList"))
    if brands:
        profile["sec-ch-ua"] = brands
    if full_versions:
        profile["sec-ch-ua-full-version-list"] = full_versions
    if browser.get("mobile") is not None:
        profile["sec-ch-ua-mobile"] = "?1" if browser["mobile"] else "?0"
    for source, target in _CLIENT_HINT_NAMES:
        value = browser.get(source)
        if isinstance(value, str):
            profile[target] = f'"{value.replace(chr(34), "")}"'
    if isinstance(browser.get("formFactors"), list):
        profile["sec-ch-ua-form-factors"] = f'"{";".join(browser["formFactors"])}"'
    if browser.get("wow64") is not None:
        profile["sec-ch-ua-wow64"] = "?1" if browser["wow64"] else "?0"


def _format_brands(brands: Any) -> str | None:
    if not isinstance(brands, list):
        return None
    formatted = []
    for item in brands:
        if not isinstance(item, dict):
            continue
        brand, version = item.get("brand"), item.get("version")
        if isinstance(brand, str) and isinstance(version, str):
            formatted.append(f'"{brand.replace(chr(34), "")}";v="{version.replace(chr(34), "")}"')
    return ", ".join(formatted) or None


_CLIENT_HINT_NAMES = (
    ("architecture", "sec-ch-ua-arch"),
    ("bitness", "sec-ch-ua-bitness"),
    ("model", "sec-ch-ua-model"),
    ("platform", "sec-ch-ua-platform"),
    ("platformVersion", "sec-ch-ua-platform-version"),
)
