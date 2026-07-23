"""Coordinate validation, browser state and extension token snapshots."""

import asyncio
import os
from typing import Any

from ..broker import TokenBroker
from ..browser.fleet import BrowserFleet
from ..events import emit
from ..validation import validate_token_request
from .guards import assert_generate_fingerprint, assert_session_matches


class TokenService:
    def __init__(self, broker: TokenBroker, browsers: BrowserFleet):
        self.broker = broker
        self.browsers = browsers
        self._locks: dict[str, asyncio.Lock] = {}
        self._activated_sessions: dict[str, str] = {}

    async def create(self, body: dict[str, Any]) -> dict[str, Any]:
        if body.get("attestationEnabled") is False:
            return {"token": ""}
        browser_id = self.browsers.resolve(body.get("browserId"))
        lock = self._locks.setdefault(browser_id, asyncio.Lock())
        async with lock:
            return await self._create(body, browser_id)

    async def _create(
        self,
        body: dict[str, Any],
        browser_id: str,
    ) -> dict[str, Any]:
        headers, auth_user = validate_token_request(body)
        self.browsers.assert_identity(browser_id, body["cookies"], auth_user)
        browser = self.browsers.session(browser_id)
        session = await browser.prepare(body["cookies"], auth_user)
        assert_generate_fingerprint(headers, session)

        extension_result = await self._snapshot(body["digest"], auth_user, browser_id)
        session_id = browser.fingerprint or ""
        needs_activation = self._activated_sessions.get(browser_id) != session_id
        if os.getenv("TOKEN_FACTORY_SAME_BROWSER_PROBE") == "1" and needs_activation:
            provider_index = await self._select_provider(
                body,
                extension_result,
                browser_id,
            )
            extension_result = await self._snapshot(
                body["digest"],
                auth_user,
                browser_id,
                provider_index,
            )
            self._activated_sessions[browser_id] = session_id

        current = await browser.snapshot()
        assert_session_matches(body["cookies"], current["cookieRecords"])
        runtime = {
            **session["runtimeConfig"],
            **extension_result.get("runtimeConfig", {}),
            "authUser": auth_user,
        }
        result = {
            "token": extension_result["token"],
            "cookieRecords": current["cookieRecords"],
            "transportProfile": current["transportProfile"],
            "runtimeConfig": runtime,
            "browserId": browser_id,
        }
        emit(
            "token-created",
            browserId=browser_id,
            digestPrefix=f"{body['digest'][:8]}…",
            tokenLength=len(result["token"]),
            cookieCount=len(result["cookieRecords"]),
        )
        return result

    async def _snapshot(
        self,
        digest: str,
        auth_user: str,
        browser_id: str,
        provider_index: int | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {"digest": digest, "authUser": auth_user}
        if provider_index is not None:
            payload["providerIndex"] = provider_index
        result = await self.broker.request(payload, browser_id)
        if not isinstance(result.get("token"), str) or not result["token"]:
            raise RuntimeError("Container extension returned an empty token")
        return result

    async def _select_provider(
        self,
        body: dict[str, Any],
        extension_result: dict[str, Any],
        browser_id: str,
    ) -> int:
        candidates = extension_result.get("candidateTokens")
        if not isinstance(candidates, list) or not candidates:
            candidates = [extension_result["token"]]
        for index, token in enumerate(candidates):
            probe = await self.browsers.session(browser_id).probe(
                body["generateRequest"],
                token,
            )
            emit(
                "same-browser-provider-probe",
                browserId=browser_id,
                providerIndex=index,
                providerCount=len(candidates),
                tokenLength=len(token),
                status=probe.get("status"),
                networkError=probe.get("networkError"),
                responseBytes=len(probe.get("body", "").encode()),
            )
            if probe.get("status") and probe["status"] not in (401, 403):
                return index
        raise RuntimeError("No native provider was accepted by the same-browser probe")
