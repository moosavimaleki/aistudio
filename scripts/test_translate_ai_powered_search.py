import os
import sys
import unittest
from unittest.mock import patch

from examples import translate_ai_powered_search as translator


class TranslationBrowserSelectionTest(unittest.TestCase):
    def test_browser_is_automatically_selected_by_default(self) -> None:
        with patch.dict(os.environ, {}, clear=True), patch.object(
            sys, "argv", ["translate_ai_powered_search.py"]
        ):
            self.assertEqual(translator.parse_args().browser_id, "")

    def test_session_pins_the_browser_returned_by_backend(self) -> None:
        metadata = {
            "conversation_id": "conversation-1",
            "parent_message_id": "message-1",
            "browser_id": "chatgpt3",
        }
        with patch.object(translator, "complete", return_value=("READY", metadata)):
            session = translator.create_session("http://localhost", 1, "test-model", "")

        self.assertEqual(session.browser_id, "chatgpt3")


if __name__ == "__main__":
    unittest.main()
