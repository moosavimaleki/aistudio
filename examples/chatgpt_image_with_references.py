"""ساخت تصویر با آپلود عکس‌های مرجع از طریق API سازگار با OpenAI."""

import argparse
import base64
import json
import mimetypes
import os
from pathlib import Path
import urllib.error
import urllib.request
import uuid


def multipart_body(fields, images, boundary):
    chunks = []
    marker = boundary.encode()
    for name, value in fields.items():
        chunks.append(b"--" + marker + b"\r\n")
        chunks.append(f'Content-Disposition: form-data; name="{name}"\r\n\r\n'.encode())
        chunks.extend((value.encode(), b"\r\n"))

    for image_path in images:
        content_type = mimetypes.guess_type(image_path.name)[0] or "application/octet-stream"
        chunks.append(b"--" + marker + b"\r\n")
        chunks.append(
            f'Content-Disposition: form-data; name="image"; filename="{image_path.name}"\r\n'.encode()
        )
        chunks.append(f"Content-Type: {content_type}\r\n\r\n".encode())
        chunks.extend((image_path.read_bytes(), b"\r\n"))

    chunks.append(b"--" + marker + b"--\r\n")
    return b"".join(chunks)


def main():
    parser = argparse.ArgumentParser(description="Generate an image using uploaded reference images.")
    parser.add_argument("images", nargs="+", type=Path, help="Reference image paths (up to four)")
    parser.add_argument("--prompt", required=True, help="Image generation prompt")
    parser.add_argument("--output", type=Path, default=Path("chatgpt-image.png"))
    parser.add_argument("--browser-id", default=os.getenv("CHATGPT_BROWSER_ID", ""))
    args = parser.parse_args()

    if len(args.images) > 4:
        parser.error("at most four reference images are supported")
    for image_path in args.images:
        if not image_path.is_file():
            parser.error(f"image file does not exist: {image_path}")

    fields = {
        "model": "chatgpt-web",
        "prompt": args.prompt,
        "n": "1",
        "response_format": "b64_json",
    }
    if args.browser_id:
        fields["browser_id"] = args.browser_id

    boundary = "----aistudio-" + uuid.uuid4().hex
    body = multipart_body(fields, args.images, boundary)
    base_url = os.getenv("OPENAI_BASE_URL", "http://127.0.0.1:3346").rstrip("/")
    request = urllib.request.Request(
        f"{base_url}/v1/images/generations",
        data=body,
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(request, timeout=float(os.getenv("OPENAI_TIMEOUT", "420"))) as response:
            result = json.load(response)
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", errors="replace")
        raise SystemExit(f"Image generation failed with HTTP {error.code}: {detail}") from error

    try:
        image = result["data"][0]["b64_json"]
    except (KeyError, IndexError, TypeError) as error:
        raise SystemExit(f"API response contains no generated image: {result}") from error

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(base64.b64decode(image, validate=True))
    print(args.output.resolve())
    metadata = result.get("lab_metadata", {})
    if metadata.get("browser_id"):
        print(f"browser_id={metadata['browser_id']} reference_count={metadata.get('reference_count', len(args.images))}")


if __name__ == "__main__":
    main()
