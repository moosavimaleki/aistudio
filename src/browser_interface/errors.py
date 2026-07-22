"""Domain errors exposed by the browser interface."""


class InvalidCookieSession(RuntimeError):
    """The supplied Google cookie session cannot open AI Studio."""


class UnknownBrowser(ValueError):
    """The request selected a browserId that is not configured."""


class BrowserIdentityMismatch(ValueError):
    """Incoming cookies do not belong to the selected browser account."""
