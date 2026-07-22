"""Prime AI Studio's native provider lifecycle without sending inference."""

import asyncio
import json
from typing import Any

from ..events import emit


async def prime_native_generate(context: Any, page: Any) -> None:
    if page.is_closed():
        raise RuntimeError(
            "Container AI Studio page is not ready for native lifecycle initialization"
        )

    captured = asyncio.get_running_loop().create_future()

    async def intercept(route: Any) -> None:
        request = route.request
        if "MakerSuiteService/GenerateContent" not in request.url:
            await route.continue_()
            return
        try:
            payload = json.loads(request.post_data or "null")
            if not isinstance(payload, list) or len(payload) < 5 or not payload[4]:
                raise RuntimeError("Native AI Studio lifecycle produced no attestation token")
            await route.abort("blockedbyclient")
            if not captured.done():
                captured.set_result(None)
        except Exception as error:
            if not captured.done():
                captured.set_exception(error)

    await context.route("**/*", intercept)
    try:
        await _accept_consent(page)
        prompt = page.get_by_role("textbox", name="Enter a prompt")
        await prompt.wait_for(state="visible", timeout=30_000)
        await prompt.fill("آزمون آماده‌سازی داخلی")
        run = page.locator('button[type="submit"]').filter(has_text="Run")
        await run.wait_for(state="visible", timeout=30_000)
        await run.evaluate("element => element.click()")
        await asyncio.wait_for(captured, timeout=45)
        emit("browser-native-lifecycle-ready")
    finally:
        await context.unroute("**/*", intercept)


async def _accept_consent(page: Any) -> None:
    consent = page.get_by_role("button", name="Agree", exact=True)
    if await consent.count():
        try:
            await consent.click()
            await page.wait_for_timeout(500)
        except Exception:
            pass
