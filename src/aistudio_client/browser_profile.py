"""Observed browser identity headers used by every staging RPC."""

from __future__ import annotations

from shared import upstream_value

DEFAULT_BROWSER_IDENTITY_HEADERS = {
    "Accept-Language": "en-US,en;q=0.9,fa;q=0.8,cs;q=0.7",
    "User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",
    "sec-ch-ua": '"Not;A=Brand";v="8", "Chromium";v="150", "Google Chrome";v="150"',
    "sec-ch-ua-arch": '"x86"', "sec-ch-ua-bitness": '"64"',
    "sec-ch-ua-form-factors": '"Desktop"',
    "sec-ch-ua-full-version": '"150.0.7871.124"',
    "sec-ch-ua-full-version-list": '"Not;A=Brand";v="8.0.0.0", "Chromium";v="150.0.7871.124", "Google Chrome";v="150.0.7871.124"',
    "sec-ch-ua-mobile": "?0", "sec-ch-ua-model": '""', "sec-ch-ua-platform": '"Linux"',
    "sec-ch-ua-platform-version": '""', "sec-ch-ua-wow64": "?0",
    "X-Browser-Channel": "stable", "X-Browser-Copyright": "Copyright 2026 Google LLC. All Rights Reserved.",
    "X-Browser-Validation": upstream_value("opaque", "x_browser_validation"),
    "X-Browser-Year": "2026",
    "X-Client-Data": upstream_value("opaque", "x_client_data"),
}


def create_browser_transport_profile() -> dict[str, str]:
    return {
        "Accept": "*/*", **DEFAULT_BROWSER_IDENTITY_HEADERS,
        "Priority": "u=1, i", "sec-fetch-dest": "empty",
        "sec-fetch-mode": "cors", "sec-fetch-site": "same-site",
    }


BOOTSTRAP_HEADERS = {
    "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
    "Accept-Encoding": "gzip, deflate, br, zstd", "Accept-Language": DEFAULT_BROWSER_IDENTITY_HEADERS["Accept-Language"],
    "Cache-Control": "max-age=0", "Priority": "u=0, i", "Upgrade-Insecure-Requests": "1",
    "User-Agent": DEFAULT_BROWSER_IDENTITY_HEADERS["User-Agent"],
    **{key: value for key, value in DEFAULT_BROWSER_IDENTITY_HEADERS.items() if key.startswith("sec-ch-") or key.startswith("X-Browser") or key == "X-Client-Data"},
    "sec-fetch-dest": "document", "sec-fetch-mode": "navigate", "sec-fetch-site": "same-origin", "sec-fetch-user": "?1",
}
