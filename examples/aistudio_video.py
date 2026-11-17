"""Generate an AI Studio video through the local API and download its MP4."""

import argparse
import base64
import json
import os
from pathlib import Path
import tempfile
import urllib.error
import urllib.parse
import urllib.request


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default=os.getenv("AISTUDIO_GENCONTENT_URL", "http://127.0.0.1:3346"))
    parser.add_argument("--model", default="gemini-omni-1.1-flash")
    parser.add_argument("--browser-id", default=os.getenv("AISTUDIO_BROWSER_ID", ""))
    parser.add_argument("--prompt", default="Create a short video of a little cartoon elephant happily jumping in a colorful garden.")
    parser.add_argument("--output", type=Path, default=Path("aistudio-video.mp4"))
    parser.add_argument("--response-format", choices=("url", "b64_json"), default="url")
    parser.add_argument("--timeout", type=float, default=510)
    args = parser.parse_args()
    base_url = args.base_url.rstrip("/")
    payload = {
        "model": args.model,
        "prompt": args.prompt,
        "browser_id": args.browser_id,
        "response_format": args.response_format,
        "n": 1,
    }
    request = urllib.request.Request(
        base_url + "/v1/videos/generations",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    print("Generating video; this can take several minutes...", flush=True)
    try:
        with urllib.request.urlopen(request, timeout=args.timeout) as response:
            result = json.load(response)
        if result.get("error"):
            raise RuntimeError(json.dumps(result["error"], ensure_ascii=False))
        item = result["data"][0]
        if args.response_format == "b64_json":
            data = base64.b64decode(item["b64_json"], validate=True)
        else:
            download_url = urllib.parse.urljoin(base_url + "/", item["url"])
            if urllib.parse.urlsplit(download_url).netloc != urllib.parse.urlsplit(base_url).netloc:
                raise RuntimeError("Video download URL points to a different host")
            with urllib.request.urlopen(download_url, timeout=args.timeout) as response:
                data = response.read()
    except urllib.error.HTTPError as error:
        detail = error.read(16384).decode(errors="replace")
        raise SystemExit(f"Video request failed with HTTP {error.code}: {detail}") from error
    if len(data) < 24 or data[4:8] != b"ftyp":
        raise SystemExit("API returned no valid MP4; output file was not changed")
    if len(data) != item["size_bytes"]:
        raise SystemExit("Incomplete MP4 download; output file was not changed")
    output = args.output.expanduser().resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=output.parent, suffix=".mp4", delete=False) as temporary:
        temporary.write(data)
        temporary_path = Path(temporary.name)
    temporary_path.replace(output)
    print(f"Saved {len(data):,} bytes to {output}")
    print(json.dumps({"usage": result.get("usage"), "metadata": result.get("lab_metadata")}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
