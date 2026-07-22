import unittest

from aistudio_client.http import _is_local


class HttpRoutingTests(unittest.TestCase):
    def test_docker_service_name_bypasses_external_proxy(self):
        self.assertTrue(_is_local("http://browser-interface:3345/bootstrap"))
        self.assertTrue(_is_local("http://localhost:3345/bootstrap"))
        self.assertFalse(_is_local("https://aistudio.google.com/prompts/new_chat"))


if __name__ == "__main__":
    unittest.main()
