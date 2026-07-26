import unittest
from unittest.mock import Mock

from aistudio_client.http import HttpClient, _is_local


class HttpRoutingTests(unittest.TestCase):
    def test_docker_service_name_bypasses_external_proxy(self):
        self.assertTrue(_is_local("http://browser-interface:3345/bootstrap"))
        self.assertTrue(_is_local("http://localhost:3345/bootstrap"))
        self.assertFalse(_is_local("https://aistudio.google.com/prompts/new_chat"))

    def test_request_can_override_default_timeout(self):
        client = HttpClient(None, timeout=60)
        client.session.request = Mock(return_value=Mock(status_code=200))

        client.request("PUT", "https://example.test/upload", timeout=180)

        self.assertEqual(client.session.request.call_args.kwargs["timeout"], 180)


if __name__ == "__main__":
    unittest.main()
