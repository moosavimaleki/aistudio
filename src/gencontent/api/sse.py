"""پاسخ Vertex را به یک event استاندارد SSE تبدیل می‌کند."""

import json
from collections.abc import Iterable


def vertex_sse(response: dict) -> Iterable[str]:
    # SDK رسمی هر قطعه را از یک خط data و یک خط خالی می‌خواند.
    yield f"data: {json.dumps(response, ensure_ascii=False)}\n\n"
