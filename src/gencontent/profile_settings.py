"""تنظیمات client را با cookie همان Chrome انتخاب‌شده هماهنگ می‌کند."""

from dataclasses import replace
from aistudio_client.config import Settings, parse_netscape_cookie_header
from aistudio_client.cookie_files import discover_cookie_files
from aistudio_client.errors import ClientError

from .profiles import BrowserProfile


class ProfileSettings:
    def __init__(self, base: Settings) -> None:
        self.base = base

    def build(self, profile: BrowserProfile) -> Settings:
        return replace(
            self.base,
            browser_id=profile.browser_id,
            auth_user=profile.auth_user,
            cookie_header=self._cookie_header(profile.slot),
        )

    def _cookie_header(self, slot: int) -> str:
        directory = self.base.values.get("AISTUDIO_COOKIE_DIR", "")
        try:
            path = discover_cookie_files(directory)[slot - 1]
            return parse_netscape_cookie_header(path.read_text(encoding="utf-8"))
        except Exception as error:
            raise ClientError(f"Cannot load cookie profile {slot}: {error}", phase="CONFIG") from error
