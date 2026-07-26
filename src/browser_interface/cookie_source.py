"""Read a cookie header or Netscape cookie export."""

from pathlib import Path


def read_cookie_file(path: str) -> str:
    text = Path(path).read_text(encoding="utf-8")
    records = _parse_netscape(text)
    return "; ".join(f"{name}={value}" for name, value in records) or text.strip()


def _parse_netscape(text: str) -> list[tuple[str, str]]:
    cookies = []
    for source_line in text.splitlines():
        line = source_line.strip()
        if line.startswith("#HttpOnly_"):
            line = line.removeprefix("#HttpOnly_")
        elif not line or line.startswith("#"):
            continue
        fields = line.split("\t")
        if len(fields) < 7 or not _is_google_domain(fields[0]):
            continue
        name, value = fields[5].strip(), fields[6].strip()
        if name and value:
            cookies.append((name, value))
    return cookies


def _is_google_domain(domain: str) -> bool:
    normalized = domain.removeprefix(".").lower()
    return normalized == "google.com" or normalized.endswith(".google.com")
