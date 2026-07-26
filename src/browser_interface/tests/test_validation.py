import hashlib
from unittest import TestCase

from browser_interface.validation import compute_content_digest, validate_token_request
from shared import upstream_value


class ValidationTests(TestCase):
    def test_digest_includes_drive_file_id(self):
        payload = ["models/test", [[[
            [None, None, None, None, None, ["drive-file-1"]],
            [None, "رونویسی کن"],
        ], "user"]]]

        expected = hashlib.sha256("drive-file-1 رونویسی کن".encode()).hexdigest()
        self.assertEqual(compute_content_digest(payload), expected)

    def test_digest_uses_generate_content_text_projection(self):
        payload = ["models/test", [[[[None, "one"], [None, "two"]], "user"]]]
        self.assertRegex(compute_content_digest(payload), r"^[a-f0-9]{64}$")

    def test_valid_request_binds_essential_metadata(self):
        headers, auth_user = validate_token_request(_valid_body())
        self.assertEqual(auth_user, "1")
        self.assertEqual(headers["user-agent"], "Chrome/136")

    def test_different_payload_is_rejected(self):
        body = _valid_body()
        body["generateRequest"]["payload"][1][0][0][0][1] = "متن دیگر"
        with self.assertRaisesRegex(ValueError, "does not match digest"):
            validate_token_request(body)

    def test_different_cookie_context_is_rejected(self):
        body = _valid_body()
        body["generateRequest"]["headers"]["Cookie"] = "SAPISID=other"
        with self.assertRaisesRegex(ValueError, "cookie context differs"):
            validate_token_request(body)

    def test_different_waa_key_is_rejected(self):
        body = _valid_body()
        body["waaApiKey"] = "wrong"
        with self.assertRaisesRegex(ValueError, "upstream config"):
            validate_token_request(body)


def _valid_body() -> dict:
    payload = ["models/test", [[[[None, "سلام"]], "user"]], [], [], None]
    cookies = "SID=sid; SAPISID=sapisid"
    return {
        "digest": compute_content_digest(payload),
        "cookies": cookies,
        "authorization": "SAPISIDHASH proof",
        "waaApiKey": upstream_value("opaque", "waa_api_key"),
        "authUser": "1",
        "generateRequest": {
            "method": "POST",
            "url": (
                "https://host/"
                f'{upstream_value("makersuite", "service")}/GenerateContent'
            ),
            "headers": {
                "Authorization": "SAPISIDHASH proof",
                "Cookie": cookies,
                "Origin": "https://aistudio.google.com",
                "User-Agent": "Chrome/136",
                "X-Client-Data": "client-data",
                "X-Goog-Api-Key": "runtime-key",
                "X-Goog-AuthUser": "1",
            },
            "payload": payload,
        },
    }
