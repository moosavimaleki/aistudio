import unittest
from types import SimpleNamespace
from unittest.mock import Mock

from aistudio_client.errors import ClientError
from aistudio_client.models import GenerateResult
from gencontent.observability.service import ObservedGenerateService


class ObservedGenerateServiceTests(unittest.TestCase):
    def test_records_success_without_changing_result(self):
        result = SimpleNamespace(
            browser_id="default",
            result=GenerateResult(final_text="ok", chunks=[["chunk"]]),
        )
        service = Mock()
        service.generate.return_value = result
        metrics = Mock()
        observed = ObservedGenerateService(service, metrics)
        input = SimpleNamespace(
            model="models/flash",
            contents=[{"parts": [{"fileData": {"fileId": "file-1"}}]}],
        )

        self.assertIs(observed.generate(input), result)
        metrics.increment.assert_any_call(
            "generate.result",
            labels={"model": "flash", "outcome": "success"},
        )
        metrics.increment.assert_any_call(
            "attachment.part",
            1,
            {"model": "flash", "kind": "reused"},
        )

    def test_records_safe_error_dimensions(self):
        service = Mock()
        service.generate.side_effect = ClientError(
            "secret response",
            phase="AUTH",
            status=401,
        )
        metrics = Mock()
        observed = ObservedGenerateService(service, metrics)
        input = SimpleNamespace(model="models/flash", contents=[])

        with self.assertRaises(ClientError):
            observed.generate(input)

        metrics.event.assert_called_once_with(
            "generate",
            "generate-error",
            model="flash",
            phase="AUTH",
            status="401",
            errorType="ClientError",
        )


if __name__ == "__main__":
    unittest.main()
