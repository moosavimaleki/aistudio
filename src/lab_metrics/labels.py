"""نام metric و labelهای کم‌کاردینالیتی را encode/decode می‌کند."""

from urllib.parse import quote, unquote


def metric_field(name: str, labels: dict[str, object] | None = None) -> str:
    clean_name = _clean(name, 80)
    if not labels:
        return clean_name
    encoded = "&".join(
        f"{quote(_clean(key, 32), safe='')}={quote(dimension(value), safe='')}"
        for key, value in sorted(labels.items())
        if value is not None
    )
    return f"{clean_name}|{encoded}" if encoded else clean_name


def parse_field(value: str | bytes) -> tuple[str, dict[str, str]]:
    text = value.decode() if isinstance(value, bytes) else str(value)
    name, separator, encoded = text.partition("|")
    labels: dict[str, str] = {}
    if separator:
        for pair in encoded.split("&"):
            key, found, item = pair.partition("=")
            if found:
                labels[unquote(key)] = unquote(item)
    return name, labels


def dimension(value: object, limit: int = 80) -> str:
    return _clean(str(value or "unknown"), limit)


def _clean(value: str, limit: int) -> str:
    cleaned = "".join(
        character if character.isalnum() or character in "._-:/" else "_"
        for character in value.strip()
    )
    return (cleaned or "unknown")[:limit]
