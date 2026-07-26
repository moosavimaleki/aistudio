"""In-memory request/reply queue consumed by the Chrome extension."""

import asyncio
import time
from dataclasses import dataclass
from typing import Any
from uuid import uuid4


@dataclass
class _Job:
    browser_id: str
    payload: dict[str, Any]
    future: asyncio.Future[dict[str, Any]]
    dispatched_at: float = 0


class TokenBroker:
    def __init__(self, timeout: float = 60, redispatch_after: float = 10):
        self.timeout = timeout
        self.redispatch_after = redispatch_after
        self._jobs: dict[str, _Job] = {}
        self._heartbeats: dict[str, float] = {}

    async def request(
        self,
        payload: dict[str, Any],
        browser_id: str = "default",
    ) -> dict[str, Any]:
        job_id = str(uuid4())
        future = asyncio.get_running_loop().create_future()
        self._jobs[job_id] = _Job(
            browser_id=browser_id,
            payload=payload,
            future=future,
        )
        try:
            return await asyncio.wait_for(future, timeout=self.timeout)
        except TimeoutError as error:
            raise RuntimeError(
                "Container extension did not return a token before timeout"
            ) from error
        finally:
            self._jobs.pop(job_id, None)

    def next(self, browser_id: str = "default") -> dict[str, Any] | None:
        now = time.monotonic()
        self._heartbeats[browser_id] = now
        for job_id, job in self._jobs.items():
            available = not job.dispatched_at or now - job.dispatched_at >= self.redispatch_after
            if job.browser_id == browser_id and available:
                job.dispatched_at = now
                return {"id": job_id, **job.payload}
        return None

    def complete(
        self,
        job_id: str,
        result: dict[str, Any],
        browser_id: str = "default",
    ) -> bool:
        job = self._jobs.get(job_id)
        if not job or job.browser_id != browser_id or job.future.done():
            return False
        self._jobs.pop(job_id, None)
        message = result.get("error")
        if isinstance(message, str) and message:
            job.future.set_exception(RuntimeError(message))
        else:
            job.future.set_result(result)
        return True

    def health(self, browser_id: str | None = None) -> dict[str, Any]:
        if browser_id is not None:
            return self._browser_health(browser_id)
        connected = [self._browser_health(item)["connected"] for item in self._heartbeats]
        return {
            "connected": bool(connected) and all(connected),
            "pendingJobs": len(self._jobs),
        }

    def _browser_health(self, browser_id: str) -> dict[str, Any]:
        last_heartbeat = self._heartbeats.get(browser_id, 0)
        heartbeat_age = time.monotonic() - last_heartbeat if last_heartbeat else None
        pending = sum(job.browser_id == browser_id for job in self._jobs.values())
        return {
            "connected": heartbeat_age is not None and heartbeat_age < 5,
            "pendingJobs": pending,
            "heartbeatAgeSeconds": round(heartbeat_age, 3) if heartbeat_age is not None else None,
        }
