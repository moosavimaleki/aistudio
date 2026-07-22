"""Small machine-readable CLI used by the E2E shell wrapper."""

from __future__ import annotations

import argparse
import json
import sys

from .client import AIStudioClient
from .errors import ClientError
from .models import GenerateInput


def main() -> int:
    parser = argparse.ArgumentParser(description="Send one prompt through the AI Studio staging client")
    parser.add_argument("prompt", nargs="?", default="سلام")
    parser.add_argument("--env-file")
    parser.add_argument("--model")
    args = parser.parse_args()
    client = AIStudioClient(env_file=args.env_file)
    model = args.model or client.settings.model
    if not model:
        raise SystemExit("AISTUDIO_MODEL is required in .env")
    try:
        print(json.dumps({"event": "initialize", "model": model}, ensure_ascii=False))
        client.initialize()
        print(json.dumps({"event": "generate", "prompt": args.prompt}, ensure_ascii=False))
        result = client.generate(GenerateInput(model=model, prompt=args.prompt))
        print(json.dumps({"event": "result", "state": client.state, "text": result.final_text, "finishReason": result.finish_reason, "chunkCount": len(result.chunks)}, ensure_ascii=False))
        return 0
    except Exception as error:
        payload = {"event": "error", "name": type(error).__name__, "message": str(error)}
        if isinstance(error, ClientError):
            payload.update({"phase": error.phase, "status": error.status, "responseBody": error.response_body, "bootstrapDiagnostics": error.diagnostics})
        print(json.dumps(payload, ensure_ascii=False), file=sys.stderr)
        return 1
    finally:
        client.close()


if __name__ == "__main__":
    raise SystemExit(main())
