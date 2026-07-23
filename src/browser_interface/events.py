"""Small JSON event logger for container diagnostics."""

import json
from typing import Any, Callable


_sink: Callable[[str, dict], None] | None = None


def set_sink(sink: Callable[[str, dict], None] | None) -> None:
    global _sink
    _sink = sink


def emit(event: str, **fields: Any) -> None:
    print(json.dumps({"event": event, **fields}, ensure_ascii=False), flush=True)
    if _sink:
        try:
            _sink(event, fields)
        except Exception:
            # observability هرگز نباید جریان اصلی browser را خراب کند.
            pass
