from unittest import TestCase
from unittest.mock import Mock

from aistudio_client.media.resumable import CHUNK_BYTES, upload_resumable


class ResumableUploadTests(TestCase):
    def test_large_content_is_sent_in_multiple_chunks(self):
        initiation = _response(200, headers={"Location": "https://upload.test/session"})
        pending = _response(308)
        completed = _response(200, json_value={"id": "file-1"})
        tab = _tab(initiation, pending, completed)
        content = b"x" * (CHUNK_BYTES + 5)

        result = upload_resumable(
            tab,
            url="https://drive.test/files?uploadType=resumable",
            headers={"Authorization": "Bearer token"},
            metadata={"name": "voice.mp3", "parents": ["folder"]},
            mime_type="audio/mpeg",
            content=content,
        )

        self.assertIs(result, completed)
        first_chunk = tab.http.request.call_args_list[1]
        last_chunk = tab.http.request.call_args_list[2]
        self.assertEqual(first_chunk.kwargs["headers"]["Content-Range"], f"bytes 0-{CHUNK_BYTES - 1}/{len(content)}")
        self.assertEqual(last_chunk.kwargs["headers"]["Content-Range"], f"bytes {CHUNK_BYTES}-{len(content) - 1}/{len(content)}")


def _tab(*responses):
    tab = Mock()
    tab.http.request.side_effect = responses
    return tab


def _response(status: int, *, headers=None, json_value=None):
    response = Mock()
    response.status_code = status
    response.ok = status < 400
    response.headers = headers or {}
    response.text = ""
    response.json.return_value = json_value or {}
    return response
