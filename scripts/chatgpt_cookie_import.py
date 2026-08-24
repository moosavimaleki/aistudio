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


# This is the smallest authenticated read observed in chatgpt.com_only_chat.har.
# Unlike /api/auth/session, it is served by the protected backend that actually
# accepts the cookie session used for chats. limit=1 keeps the validation cheap.
CONVERSATIONS_URL = (
    "https://chatgpt.com/backend-api/conversations?offset=0&limit=1"
    "&order=updated&is_archived=false&is_starred=false"
)
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
    request = Request(
        CONVERSATIONS_URL,
        headers={
            "Accept": "*/*",
            "Accept-Language": "en-US,en;q=0.9",
            "Referer": "https://chatgpt.com/",
            "Oai-Device-Id": str(uuid.uuid4()),
            "Oai-Language": "en-US",
            "X-Openai-Target-Path": "/backend-api/conversations",
            "X-Openai-Target-Route": "/backend-api/conversations",
            "User-Agent": (
                "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
                "(KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
            ),
        },
    )
    try:
        with build_opener(*handlers).open(request, timeout=timeout) as response:
            payload = json.loads(response.read())
    except HTTPError as error:
        return False, f"conversations endpoint returned HTTP {error.code}"
    except (OSError, URLError, UnicodeError, json.JSONDecodeError) as error:
        return False, f"conversations check failed: {type(error).__name__}"
    if not _is_conversation_page(payload):
        return False, "conversations response was not an authenticated page"
    return True, "active backend session"


def _is_conversation_page(payload: object) -> bool:
    """Recognize the paginated response shape captured from ChatGPT's backend."""
    if not isinstance(payload, dict):
        return False
    if not isinstance(payload.get("items"), list):
        return False
    return all(isinstance(payload.get(key), int) for key in ("total", "limit", "offset"))


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
