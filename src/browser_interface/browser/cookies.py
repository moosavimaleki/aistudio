"""Translate and verify Google cookie material."""

import hashlib
from typing import Any

AUTH_COOKIE_NAMES = (
    "SAPISID",
    "__Secure-1PAPISID",
    "__Secure-3PAPISID",
)

SESSION_COOKIE_NAMES = ("SID", *AUTH_COOKIE_NAMES)


def parse_google_cookies(cookie_header: str) -> list[dict[str, Any]]:
    records = []
    for pair in str(cookie_header or "").split(";"):
        if "=" not in pair:
            continue
        name, value = (part.strip() for part in pair.split("=", 1))
        if not name or not value or name.startswith("__Host-"):
            continue
        records.append(
            {
                "name": name,
                "value": value,
                "domain": ".google.com",
                "path": "/",
                "secure": True,
            }
        )
    return records


def google_cookie_records(cookies: list[dict[str, Any]]) -> list[dict[str, str]]:
    records = []
    for cookie in cookies or []:
        domain = str(cookie.get("domain", "")).removeprefix(".").lower()
        if domain != "google.com" and not domain.endswith(".google.com"):
            continue
        name, value = cookie.get("name"), cookie.get("value")
        if name and value:
            records.append({"name": str(name), "value": str(value)})
    return records


def session_fingerprint(cookie_header: str, auth_user: str) -> str:
    selected = {
        record["name"]: record["value"]
        for record in parse_google_cookies(cookie_header)
    }
    material = "\0".join(
        str(value or "")
        for value in (
            auth_user,
            selected.get("SID"),
            selected.get("SAPISID"),
            selected.get("__Secure-1PAPISID"),
            selected.get("__Secure-3PAPISID"),
        )
    )
    return hashlib.sha256(material.encode()).hexdigest()


async def apply_and_verify(context: Any, cookies: list[dict[str, Any]]) -> None:
    await context.add_cookies(cookies)
    actual = {
        record["name"]: record["value"]
        for record in google_cookie_records(await context.cookies())
    }
    for cookie in cookies:
        if cookie["name"] not in SESSION_COOKIE_NAMES:
            continue
        if actual.get(cookie["name"]) != cookie["value"]:
            raise RuntimeError(
                f"Container Chrome did not apply incoming cookie {cookie['name']}"
            )


def authorization_cookies_match(
    cookie_header: str,
    cookie_records: list[dict[str, str]],
) -> bool:
    expected = _header_values(cookie_header)
    actual = {record["name"]: record["value"] for record in cookie_records}
    return all(
        not expected.get(name) or expected[name] == actual.get(name)
        for name in AUTH_COOKIE_NAMES
    )


def _header_values(cookie_header: str) -> dict[str, str]:
    values = {}
    for pair in str(cookie_header).split(";"):
        if "=" in pair:
            name, value = pair.split("=", 1)
            values[name.strip()] = value.strip()
    return values
