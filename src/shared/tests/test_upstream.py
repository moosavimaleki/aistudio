import unittest
from pathlib import Path

from shared import upstream_config, upstream_value


class UpstreamConfigTests(unittest.TestCase):
    def test_required_opaque_values_exist(self):
        self.assertTrue(upstream_value("opaque", "x_client_data"))
        self.assertTrue(upstream_value("opaque", "x_browser_validation"))
        self.assertTrue(upstream_value("opaque", "waa_api_key"))

    def test_extension_bundle_does_not_contain_waa_secret(self):
        bundle = Path("/app/extension/dist/page.js")
        if not bundle.is_file():
            self.skipTest("extension bundle is not built")
        self.assertNotIn(
            upstream_config()["opaque"]["waa_api_key"],
            bundle.read_text(encoding="utf-8"),
        )


if __name__ == "__main__":
    unittest.main()
