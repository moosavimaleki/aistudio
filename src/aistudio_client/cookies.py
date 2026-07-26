"""One authoritative, deterministic cookie material for auth and networking."""

from __future__ import annotations

from collections import OrderedDict
from typing import Any


class CookieJar:
    def __init__(self, cookie_header: str) -> None:
        self._cookies: OrderedDict[str, str] = OrderedDict()
        self.update_header(cookie_header)

    def update_header(self, cookie_header: str) -> None:
        for raw_pair in str(cookie_header).split(";"):
            if "=" not in raw_pair:
                continue
            name, value = raw_pair.split("=", 1)
            name, value = name.strip(), value.strip()
            if name:
                self._cookies[name] = value

    def apply_set_cookie(self, value: str) -> None:
        first = value.split(";", 1)[0]
        if "=" not in first:
            return
        name, cookie_value = (part.strip() for part in first.split("=", 1))
        if not name:
            return
        if cookie_value:
            self._cookies[name] = cookie_value
        else:
            self._cookies.pop(name, None)

    def apply_response(self, response: Any) -> None:
        raw_headers = getattr(getattr(response, "raw", None), "headers", None)
        if raw_headers and hasattr(raw_headers, "getlist"):
            for value in raw_headers.getlist("Set-Cookie"):
                self.apply_set_cookie(value)
        else:
            set_cookie = response.headers.get("set-cookie") if getattr(response, "headers", None) else None
            if set_cookie:
                self.apply_set_cookie(set_cookie)

    def apply_records(self, records: list[dict[str, str]] | None) -> None:
        for record in records or []:
            name = str(record.get("name", "")).strip()
            value = str(record.get("value", "")).strip()
            if not name:
                continue
            if value:
                self._cookies[name] = value
            else:
                self._cookies.pop(name, None)

    @property
    def header(self) -> str:
        return "; ".join(f"{name}={value}" for name, value in self._cookies.items())
