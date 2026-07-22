"""The only permitted attestation boundary for the staging client."""

from __future__ import annotations

import json
from dataclasses import dataclass, field

from .auth import AuthContext, generate_authorization_header
from .http import HttpClient
from .models import RuntimeConfig


@dataclass
class TokenSnapshot:
    token: str
    cookie_records: list[dict[str, str]] = field(default_factory=list)
    transport_profile: dict[str, str] | None = None
    runtime_config: dict | None = None
    logging_context_extension: str | None = None


class StagingTokenFactory:
    def __init__(
        self,
        http: HttpClient,
        url: str,
        waa_api_key: str,
        auth: AuthContext,
        runtime: RuntimeConfig,
        browser_id: str | None = None,
    ) -> None:
        if not url or not waa_api_key:
            raise ValueError("token factory requires URL and opaque.waa_api_key")
        self.http, self.url, self.waa_api_key, self.auth, self.runtime = http, url, waa_api_key, auth, runtime
        self.browser_id = browser_id

    def update_session(self, auth: AuthContext, runtime: RuntimeConfig) -> None:
        self.auth, self.runtime = auth, runtime

    def snapshot(self, digest: str, generate_request: dict) -> TokenSnapshot:
        if len(digest) != 64 or any(char not in "0123456789abcdef" for char in digest):
            raise ValueError("content digest must be lowercase SHA-256 hex")
        authorization = generate_authorization_header(self.auth)
        if not authorization:
            raise ValueError("No session authorization is available for token factory")
        body = {
            "digest": digest, "cookies": self.auth.cookie_header, "authorization": authorization,
            "waaApiKey": self.waa_api_key, "visitId": self.runtime.visit_id,
            "authUser": self.runtime.auth_user, "attestationEnabled": self.runtime.attestation_enabled,
            "generateRequest": generate_request,
            **({"browserId": self.browser_id} if self.browser_id else {}),
        }
        encoded_body = json.dumps(body, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        response = self.http.request("POST", self.url, headers={"Content-Type": "application/json"}, data=encoded_body)
        if not response.ok:
            from .errors import ClientError
            raise ClientError(f"Token factory failed with HTTP {response.status_code}", phase="ATTESTATION", status=response.status_code, response_body=response.text)
        result = response.json()
        token = result.get("token")
        if not isinstance(token, str) or not token:
            raise ValueError("Token factory response does not contain a token")
        return TokenSnapshot(token, result.get("cookieRecords") or [], result.get("transportProfile"), result.get("runtimeConfig"), result.get("loggingContextExtension"))
