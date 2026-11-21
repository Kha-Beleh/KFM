import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest

from check_sast import check


class PolicyTests(unittest.TestCase):
    def report(self, data):
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "report.json"
            path.write_text(json.dumps(data), encoding="utf-8")
            with contextlib.redirect_stdout(io.StringIO()):
                return check(path)

    def test_high_and_critical_block(self):
        for severity in ["HIGH", "CRITICAL", "ERROR"]:
            with self.subTest(severity=severity):
                finding = {"extra": {"severity": severity}, "path": "sample.go", "start": {"line": 1}, "check_id": "sample"}
                self.assertEqual(self.report({"results": [finding], "errors": []}), 1)

    def test_lower_severity_does_not_block(self):
        for severity in ["LOW", "MEDIUM", "INFO", "WARNING"]:
            with self.subTest(severity=severity):
                self.assertEqual(self.report({"results": [{"extra": {"severity": severity}}], "errors": []}), 0)

    def test_empty_results_pass(self):
        self.assertEqual(self.report({"results": [], "errors": []}), 0)

    def test_scan_errors_are_not_a_clean_result(self):
        with self.assertRaises(ValueError):
            self.report({"results": [], "errors": [{"message": "parse failure"}]})

    def test_missing_results_are_rejected(self):
        with self.assertRaises(ValueError):
            self.report({"errors": []})


if __name__ == "__main__":
    unittest.main()
