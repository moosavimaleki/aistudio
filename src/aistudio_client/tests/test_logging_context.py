from __future__ import annotations

import unittest

from aistudio_client.logging_context import encode_logging_context_extension


class LoggingContextTests(unittest.TestCase):
    def test_encodes_positional_protobuf(self) -> None:
        self.assertEqual(encode_logging_context_extension([2, "GH", None, 1]), "CAISAkdIIAE=")
