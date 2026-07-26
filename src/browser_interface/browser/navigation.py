"""AI Studio account URL selection."""

from urllib.parse import quote

from ..errors import InvalidCookieSession
from shared import upstream_value

DEFAULT_DOCUMENT_URL = upstream_value("aistudio", "bootstrap_url")
ACCOUNT_DOCUMENT_URL = upstream_value("aistudio", "account_bootstrap_url")


def document_url(auth_user: str = "0") -> str:
    if str(auth_user) == "0":
        return DEFAULT_DOCUMENT_URL
    account = quote(str(auth_user), safe="")
    return ACCOUNT_DOCUMENT_URL.format(auth_user=account)


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
