"""Cross-check browser identity and authentication cookies."""

from typing import Any

from ..browser.cookies import authorization_cookies_match


def assert_generate_fingerprint(
    headers: dict[str, str],
    session: dict[str, Any],
) -> None:
    browser_headers = {
        str(name).lower(): str(value)
        for name, value in session["transportProfile"].items()
    }
    for name in ("user-agent", "x-client-data"):
        if headers.get(name) != browser_headers.get(name):
            raise RuntimeError(
                f"GenerateContent {name} differs from container Chrome"
            )
    runtime = session["runtimeConfig"]
    if headers.get("x-goog-api-key") != runtime["apiKey"]:
        raise RuntimeError(
            "GenerateContent x-goog-api-key differs from container Chrome runtime"
        )
    if headers.get("x-goog-authuser") != str(runtime["authUser"]):
        raise RuntimeError(
            "GenerateContent x-goog-authuser differs from container Chrome account"
        )


def assert_session_matches(
    cookie_header: str,
    cookie_records: list[dict[str, str]],
) -> None:
    if not authorization_cookies_match(cookie_header, cookie_records):
        raise RuntimeError("Container browser authentication cookies changed")
