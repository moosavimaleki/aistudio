"""Chrome profile discovery and safe ChatGPT cookie export helpers."""

from __future__ import annotations

import json
import os
import tempfile
import time
import uuid
from dataclasses import dataclass
from http.cookiejar import Cookie, CookieJar
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import HTTPCookieProcessor, ProxyHandler, Request, build_opener

from yt_dlp.cookies import extract_cookies_from_browser


# Captured from a working ChatGPT browser session. A half-signed-in browser
# falls back to /backend-anon/me, whose 200 response lacks client_id and orgs.
SESSION_URL = "https://chatgpt.com/api/auth/session"
ACCOUNT_CHECK_URL = "https://chatgpt.com/backend-api/accounts/check/v4-2023-04-27"
ACCOUNT_URL = "https://chatgpt.com/backend-api/me"
DEFAULT_ROOTS = (
    ("chrome", Path.home() / ".config/google-chrome"),
    ("chrome", Path.home() / ".config/google-chrome-beta"),
    ("chrome", Path.home() / ".config/google-chrome-unstable"),
    ("chromium", Path.home() / ".config/chromium"),
    ("chrome", Path.home() / ".var/app/com.google.Chrome/config/google-chrome"),
    ("chromium", Path.home() / "snap/chromium/common/chromium"),
)


@dataclass(frozen=True)
class ChromeProfile:
    browser: str
    root: Path
    directory: Path
    cookie_db: Path

    @property
    def label(self) -> str:
        return f"{self.root.name}/{self.directory.name}"


class SilentLogger:
    """Prevent yt-dlp from printing cookie details or progress."""

    def debug(self, *_args, **_kwargs) -> None:
        pass

    def error(self, *_args, **_kwargs) -> None:
        pass

    def info(self, *_args, **_kwargs) -> None:
        pass

    def warning(self, *_args, **_kwargs) -> None:
        pass


def discover_profiles(custom_roots: list[Path] | None = None) -> list[ChromeProfile]:
    roots = [("chrome", path.expanduser()) for path in custom_roots] if custom_roots else DEFAULT_ROOTS
    profiles: list[ChromeProfile] = []
    seen: set[Path] = set()
    for browser, root in roots:
        if not root.is_dir():
            continue
        for directory in sorted(path for path in root.iterdir() if path.is_dir()):
            cookie_db = directory / "Network" / "Cookies"
            if not cookie_db.is_file():
                cookie_db = directory / "Cookies"
            resolved = cookie_db.resolve()
            if not cookie_db.is_file() or resolved in seen:
                continue
            seen.add(resolved)
            profiles.append(ChromeProfile(browser, root, directory, cookie_db))
    return profiles


def extract_chatgpt_cookies(profile: ChromeProfile) -> CookieJar:
    source = extract_cookies_from_browser(
        profile.browser,
        profile=str(profile.directory),
        logger=SilentLogger(),
    )
    now = time.time()
    result = CookieJar()
    for cookie in source:
        if cookie.domain.lstrip(".").lower() != "chatgpt.com":
            continue
        if cookie.expires is not None and cookie.expires <= now:
            continue
        result.set_cookie(cookie)
    return result


def validate_session(cookies: CookieJar, proxy: str, timeout: float) -> tuple[bool, str]:
    handlers = [HTTPCookieProcessor(cookies)]
    handlers.append(ProxyHandler({"http": proxy, "https": proxy}) if proxy else ProxyHandler({}))
    opener = build_opener(*handlers)
    headers = _browser_headers()
    try:
        session = _request_json(opener, SESSION_URL, headers | {"Accept": "application/json"}, timeout)
        access_token = _access_token(session)
        if not access_token:
            return False, "session access token is missing"
        accounts = _request_json(
            opener,
            ACCOUNT_CHECK_URL,
            headers
            | {
                "X-Openai-Target-Path": "/backend-api/accounts/check/v4-2023-04-27",
                "X-Openai-Target-Route": "/backend-api/accounts/check/{version}",
            },
            timeout,
        )
        account_id = _selected_account_id(accounts)
        if not account_id:
            return False, "account selection is missing"
        payload = _request_json(
            opener,
            ACCOUNT_URL,
            headers
            | {
                "Chatgpt-Account-Id": account_id,
                "Authorization": f"Bearer {access_token}",
                "X-Openai-Target-Path": "/backend-api/me",
                "X-Openai-Target-Route": "/backend-api/me",
            },
            timeout,
        )
    except HTTPError as error:
        return False, f"account endpoint returned HTTP {error.code}"
    except (OSError, URLError, UnicodeError, json.JSONDecodeError) as error:
        return False, f"account check failed: {type(error).__name__}"
    if not _is_authenticated_account(payload):
        return False, "account response is anonymous or incomplete"
    return True, "active authenticated account"


def _browser_headers() -> dict[str, str]:
    return {
        "Accept": "*/*",
        "Accept-Language": "en-US,en;q=0.9",
        "Referer": "https://chatgpt.com/",
        "Oai-Device-Id": str(uuid.uuid4()),
        "Oai-Language": "en-US",
        "User-Agent": (
            "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
            "(KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
        ),
    }


def _request_json(opener: object, url: str, headers: dict[str, str], timeout: float) -> object:
    request = Request(url, headers=headers)
    with opener.open(request, timeout=timeout) as response:
        return json.loads(response.read())


def _selected_account_id(payload: object) -> str | None:
    if not isinstance(payload, dict):
        return None
    ordering = payload.get("account_ordering")
    if not isinstance(ordering, list) or not ordering:
        return None
    account_id = ordering[0]
    return account_id if isinstance(account_id, str) and account_id else None


def _access_token(payload: object) -> str | None:
    if not isinstance(payload, dict):
        return None
    token = payload.get("accessToken")
    return token if isinstance(token, str) and token else None


def _is_authenticated_account(payload: object) -> bool:
    """Reject the smaller 200 JSON document returned by /backend-anon/me."""
    if not isinstance(payload, dict):
        return False
    if not all(isinstance(payload.get(key), str) and payload[key] for key in ("id", "email", "client_id")):
        return False
    organizations = payload.get("orgs")
    if not isinstance(organizations, dict):
        return False
    data = organizations.get("data")
    return isinstance(data, list) and any(
        isinstance(organization, dict) and isinstance(organization.get("id"), str)
        for organization in data
    )


def write_netscape(path: Path, cookies: CookieJar, replace: bool) -> None:
    if path.exists() and not replace:
        raise FileExistsError(f"{path.name} already exists; use --replace to update generated files")
    path.parent.mkdir(parents=True, exist_ok=True)
    rows = sorted((_netscape_row(cookie) for cookie in cookies), key=lambda row: row[:6])
    fd, temporary_name = tempfile.mkstemp(prefix=f".{path.name}.", suffix=".tmp", dir=path.parent)
    temporary = Path(temporary_name)
    try:
        # The project directory is group-shared with seluser inside Docker.
        os.fchmod(fd, 0o640)
        with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as output:
            output.write("# Netscape HTTP Cookie File\n")
            output.write("# Generated locally; do not commit or share this file.\n")
            for row in rows:
                output.write("\t".join(row) + "\n")
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        path.chmod(0o640)
    finally:
        temporary.unlink(missing_ok=True)


def _netscape_row(cookie: Cookie) -> tuple[str, ...]:
    domain = clean_field(cookie.domain, "chatgpt.com")
    if cookie.has_nonstandard_attr("HttpOnly"):
        domain = "#HttpOnly_" + domain
    return (
        domain,
        "TRUE" if cookie.domain.startswith(".") else "FALSE",
        clean_field(cookie.path, "/"),
        "TRUE" if cookie.secure else "FALSE",
        str(cookie.expires or 0),
        clean_field(cookie.name, ""),
        clean_field(cookie.value, ""),
    )


def clean_field(value: str | None, fallback: str) -> str:
    cleaned = (value or "").replace("\t", "").replace("\r", "").replace("\n", "")
    return cleaned or fallback
