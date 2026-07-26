import unittest
from unittest.mock import Mock

from browser_interface.browser.diagnostics import SessionDiagnostics
from browser_interface.observability import BrowserEventMetrics


class SessionDiagnosticsTests(unittest.TestCase):
    def test_exposes_cookie_metadata_without_values(self):
        diagnostics = SessionDiagnostics()
        diagnostics.record_snapshot([
            {"name": "SID", "value": "secret-1", "expires": 2_000_000_000},
            {"name": "SAPISID", "value": "secret-2", "expires": 2_000_000_100},
        ])

        result = diagnostics.snapshot()

        self.assertEqual(result["cookieCount"], 2)
        # SID session cookie است؛ فقط خانوادهٔ SAPISID برای Authorization شمرده می‌شود.
        self.assertEqual(result["authCookieCount"], 1)
        self.assertEqual(result["cookieRevision"], 1)
        self.assertNotIn("secret", str(result))

    def test_same_cookie_snapshot_does_not_increment_revision(self):
        diagnostics = SessionDiagnostics()
        cookies = [{"name": "SID", "value": "same", "expires": -1}]

        first_changed = diagnostics.record_snapshot(cookies)
        second_changed = diagnostics.record_snapshot(cookies)

        self.assertEqual(diagnostics.cookie_revision, 1)
        self.assertFalse(first_changed)
        self.assertFalse(second_changed)


class BrowserEventMetricsTests(unittest.TestCase):
    def test_cookie_event_does_not_forward_cookie_file(self):
        metrics = Mock()
        observer = BrowserEventMetrics(metrics)

        observer("cookies-persisted", {
            "browserId": "default",
            "cookieFile": "private-file.txt",
            "cookieCount": 13,
        })

        _, kwargs = metrics.event.call_args
        self.assertNotIn("cookieFile", kwargs)
        self.assertEqual(kwargs["cookieCount"], 13)


if __name__ == "__main__":
    unittest.main()
