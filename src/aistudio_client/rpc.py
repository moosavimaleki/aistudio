"""MakerSuite unary RPC calls, including response-cookie propagation."""

from __future__ import annotations

import json

from .cookies import CookieJar
from .errors import response_error
from .http import HttpClient

MAKER_SUITE_BASE = "https://alkalimakersuite-pa.clients6.google.com"
MAKER_SUITE_SERVICE = "google.internal.alkali.applications.makersuite.v1.MakerSuiteService"


def rpc_url(method: str) -> str:
    if not method.isalnum() or not method[:1].isalpha():
        raise ValueError("RPC method must be an alphanumeric method name")
    return f"{MAKER_SUITE_BASE}/$rpc/{MAKER_SUITE_SERVICE}/{method}"


def unary(http: HttpClient, cookies: CookieJar, method: str, body: list, headers: dict[str, str], *, retryable: bool = True):
    # requests computes Content-Length from bytes correctly. Passing a unicode
    # string with Persian text can otherwise truncate the JSON body at an
    # Express JSON parser when character length and UTF-8 byte length differ.
    encoded_body = json.dumps(body, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    response = http.request("POST", rpc_url(method), headers=headers, data=encoded_body, retries=4 if retryable else 0, retryable=retryable)
    cookies.apply_response(response)
    if not response.ok:
        raise response_error(response.status_code, response.text)
    return response
