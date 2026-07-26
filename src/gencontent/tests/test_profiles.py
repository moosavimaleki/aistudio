import unittest
from unittest.mock import Mock, patch

from gencontent.profiles import BrowserProfiles


class BrowserProfilesTests(unittest.TestCase):
    @patch("gencontent.profiles.requests.get")
    def test_only_ready_profiles_are_selected(self, get):
        response = Mock()
        response.json.return_value = {
            "browsers": [
                {"browserId": "default", "authUser": "0", "connected": True, "ready": True},
                {"browserId": "browser2", "authUser": "1", "connected": True, "ready": False},
            ]
        }
        get.return_value = response

        selected = BrowserProfiles("http://browser-interface:3345/get-token").choose()

        self.assertEqual(selected.browser_id, "default")
        self.assertEqual(selected.auth_user, "0")
        self.assertEqual(selected.slot, 1)

    @patch("gencontent.profiles.requests.get")
    def test_failed_profile_can_be_excluded(self, get):
        response = Mock()
        response.json.return_value = {
            "browsers": [
                {"browserId": "default", "authUser": "0", "connected": True, "ready": True},
                {"browserId": "browser2", "authUser": "0", "connected": True, "ready": True},
            ]
        }
        get.return_value = response

        selected = BrowserProfiles(
            "http://browser-interface:3345/get-token"
        ).choose({"default"})

        self.assertEqual(selected.browser_id, "browser2")


if __name__ == "__main__":
    unittest.main()
