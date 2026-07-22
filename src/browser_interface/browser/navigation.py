"""AI Studio account URL selection."""

from urllib.parse import quote

from ..errors import InvalidCookieSession

DEFAULT_DOCUMENT_URL = "https://aistudio.google.com/prompts/new_chat"


def document_url(auth_user: str = "0") -> str:
    if str(auth_user) == "0":
        return DEFAULT_DOCUMENT_URL
    account = quote(str(auth_user), safe="")
    return f"https://aistudio.google.com/u/{account}/prompts/new_chat"


async def navigate_to_account(page, auth_user: str) -> None:
    navigation_error: Exception | None = None
    for attempt in range(1, 4):
        try:
            await page.goto(
                document_url(auth_user),
                wait_until="domcontentloaded",
                timeout=30_000,
            )
            navigation_error = None
            break
        except Exception as error:
            navigation_error = error
            await page.wait_for_timeout(attempt * 500)
    if navigation_error:
        raise navigation_error
    if "accounts.google.com" in page.url:
        raise InvalidCookieSession(
            "AI Studio redirected the container browser to sign-in; cookie session is invalid"
        )
