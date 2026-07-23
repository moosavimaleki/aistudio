import unittest
from types import SimpleNamespace
from unittest.mock import Mock, patch

from aistudio_client.errors import ClientError
from aistudio_client.models import GenerateInput, GenerateResult
from aistudio_client.tab import TabState
from gencontent.service import GenerateContentService


class GenerateContentServiceTests(unittest.TestCase):
    @patch("gencontent.service.dump_tab", return_value={"state": "ready"})
    @patch("gencontent.service.resolve_inline_data")
    def test_upload_auth_failure_discards_tab_and_uses_a_new_one(
        self,
        resolve_inline_data,
        _dump_tab,
    ):
        expired = _tab("expired")
        fresh = _tab("fresh", result=GenerateResult(final_text="ok"))
        resolve_inline_data.side_effect = [
            ClientError("expired OAuth", status=401, phase="AUTH"),
            [],
        ]
        pool = Mock()
        leases = [SimpleNamespace(tab_id="lease-1"), SimpleNamespace(tab_id="lease-2")]
        pool.acquire.side_effect = leases
        service = GenerateContentService(Mock(), pool, Mock(), Mock())
        service._materialize = Mock(side_effect=[expired, fresh])

        outcome = service.generate(GenerateInput("models/test", contents=[{"parts": []}]))

        self.assertEqual(outcome.result.final_text, "ok")
        pool.discard.assert_called_once_with(leases[0])
        pool.release.assert_called_once_with(leases[1], {"state": "ready"})
        self.assertTrue(expired.closed)


def _tab(name: str, *, result: GenerateResult | None = None):
    tab = Mock()
    tab.id = name
    tab.state = TabState.READY
    tab.settings = SimpleNamespace(browser_id="default", auth_user="0")
    tab.generate_count = 0
    tab.generate.return_value = result
    tab.closed = False

    def close():
        tab.closed = True

    tab.close.side_effect = close
    return tab


if __name__ == "__main__":
    unittest.main()
