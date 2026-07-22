"""بسته‌های نهایی افزونه را از ماژول‌های کوچک و خوانا می‌سازد."""

import json
from pathlib import Path
import sys
from urllib.parse import urlsplit


EXTENSION_DIR = Path(__file__).resolve().parent
OUTPUT_DIR = EXTENSION_DIR / "dist"
sys.path.insert(0, str(EXTENSION_DIR.parent))

from shared import upstream_value

BUNDLES = {
    "page.js": (
        "shared/protocol.js",
        "page/provider-store.js",
        "page/snapshot-service.js",
        "page/main.js",
    ),
    "content.js": (
        "shared/protocol.js",
        "content/keep-alive.js",
        "content/page-channel.js",
        "content/main.js",
    ),
}


def build_bundles() -> None:
    """هر world مرورگر را به یک فایل قطعی تبدیل می‌کند."""

    OUTPUT_DIR.mkdir(exist_ok=True)
    _build_manifest()

    for output_name, source_names in BUNDLES.items():
        source_parts = [
            (EXTENSION_DIR / source_name).read_text(encoding="utf-8")
            for source_name in source_names
        ]
        if output_name == "page.js":
            source_parts.insert(0, _page_config_script())
        output_path = OUTPUT_DIR / output_name
        output_path.write_text("\n\n".join(source_parts) + "\n", encoding="utf-8")


def _page_config_script() -> str:
    # فقط قراردادهای عمومی صفحه وارد bundle می‌شوند؛ secretهای opaque وارد JS نمی‌شوند.
    config = {
        "runtimeGlobal": upstream_value("runtime", "global_object"),
        "apiKeyProperty": upstream_value("runtime", "api_key_property"),
        "visitIdProperty": upstream_value("runtime", "visit_id_property"),
        "attestationProperty": upstream_value("runtime", "attestation_enabled_property"),
        "attestationNamespace": upstream_value("attestation", "namespace"),
        "attestationEntrypoint": upstream_value("attestation", "entrypoint"),
        "digestProperty": upstream_value("attestation", "digest_property"),
    }
    return f"globalThis.AISTUDIO_UPSTREAM_CONFIG = {json.dumps(config)};"


def _build_manifest() -> None:
    origin = upstream_value("aistudio", "origin")
    hostname = urlsplit(origin).hostname
    if not hostname:
        raise ValueError("aistudio.origin must contain a hostname")
    parent_domain = ".".join(hostname.split(".")[-2:])
    source = (EXTENSION_DIR / "manifest.template.json").read_text(encoding="utf-8")
    source = source.replace("__AISTUDIO_MATCH__", f"{origin}/*")
    source = source.replace(
        "__AISTUDIO_GOOGLE_PERMISSION__",
        f"{urlsplit(origin).scheme}://*.{parent_domain}/*",
    )
    (EXTENSION_DIR / "manifest.json").write_text(source, encoding="utf-8")


if __name__ == "__main__":
    build_bundles()
