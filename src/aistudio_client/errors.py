"""Errors with a stable phase/status shape for diagnostics and CI."""

from __future__ import annotations


class ClientError(RuntimeError):
    def __init__(
        self,
        message: str,
        *,
        phase: str = "RPC",
        status: int | None = None,
        retryable: bool = False,
        response_body: str | None = None,
        diagnostics: dict | None = None,
    ) -> None:
        super().__init__(message)
        self.phase = phase
        self.status = status
        self.retryable = retryable
        self.response_body = response_body
        self.diagnostics = diagnostics


def response_error(status: int, body: str | None = None, *, phase: str = "RPC") -> ClientError:
    return ClientError(
        f"RPC failed with HTTP {status}",
        phase="AUTH" if status in (401, 403) else phase,
        status=status,
        retryable=status in (408, 429) or status >= 500,
        response_body=body,
    )
