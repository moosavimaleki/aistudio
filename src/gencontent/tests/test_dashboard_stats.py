import unittest
from dataclasses import dataclass

from gencontent.dashboard_stats import dashboard_snapshot
from lab_metrics.labels import metric_field
from lab_metrics.query import MetricWindow


@dataclass
class _Profile:
    slot: int
    browser_id: str
    auth_user: str
    connected: bool
    ready: bool


class DashboardStatsTests(unittest.TestCase):
    def test_builds_summary_models_and_error_breakdown(self):
        values = {
            metric_field("generate.request", {"model": "flash"}): 2,
            metric_field("generate.result", {"model": "flash", "outcome": "success"}): 1,
            metric_field(
                "generate.result",
                {"model": "flash", "outcome": "error", "phase": "AUTH"},
            ): 1,
            metric_field("generate.error", {"phase": "AUTH", "status": "401"}): 1,
            metric_field("generate.duration.count", {"model": "flash"}): 2,
            metric_field("generate.duration.le_1000", {"model": "flash"}): 1,
            metric_field("generate.duration.le_2500", {"model": "flash"}): 2,
            metric_field("generate.duration.le_inf", {"model": "flash"}): 2,
        }
        window = MetricWindow([values], values, {}, [], 1)
        profile = _Profile(
            slot=1,
            browser_id="default",
            auth_user="0",
            connected=True,
            ready=True,
        )

        result = dashboard_snapshot(
            window,
            1,
            {"total": 0, "available": 0, "leased": 0, "max": 10, "tabs": []},
            [profile],
        )

        self.assertEqual(result["summary"]["requests"], 2)
        self.assertEqual(result["summary"]["successRate"], 50)
        self.assertEqual(result["models"][0]["p50"], 1000)
        self.assertEqual(result["models"][0]["p95"], 2500)
        self.assertEqual(result["errorPhases"], {"AUTH": 1})


if __name__ == "__main__":
    unittest.main()
