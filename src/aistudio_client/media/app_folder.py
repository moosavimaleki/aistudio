"""شناسهٔ پوشهٔ Drive مخصوص AI Studio را از RPC می‌گیرد."""

from __future__ import annotations

import json

from ..headers import compose_makersuite_headers
from ..rpc import unary


def get_app_folder(tab) -> str:
    headers = {
        **tab.transport_profile,
        **compose_makersuite_headers(
            tab.auth,
            tab.cookies.header,
            tab.runtime,
            logging_context_extension=tab.logging_context_extension,
        ),
    }
    response = unary(tab.http, tab.cookies, "GetAppFolder", [], headers)
    tab._sync_session()
    value = response.json()
    if not isinstance(value, list) or not value or not isinstance(value[0], str):
        raise ValueError("GetAppFolder returned an invalid folder id")
    return value[0]
