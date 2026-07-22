"""بسته‌های نهایی افزونه را از ماژول‌های کوچک و خوانا می‌سازد."""

from pathlib import Path


EXTENSION_DIR = Path(__file__).resolve().parent
OUTPUT_DIR = EXTENSION_DIR / "dist"

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

    for output_name, source_names in BUNDLES.items():
        source_parts = [
            (EXTENSION_DIR / source_name).read_text(encoding="utf-8")
            for source_name in source_names
        ]
        output_path = OUTPUT_DIR / output_name
        output_path.write_text("\n\n".join(source_parts) + "\n", encoding="utf-8")


if __name__ == "__main__":
    build_bundles()
