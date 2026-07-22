"""Small JSON event logger for container diagnostics."""

import json
from typing import Any


def emit(event: str, **fields: Any) -> None:
    print(json.dumps({"event": event, **fields}, ensure_ascii=False), flush=True)
