import json
from pathlib import Path
import subprocess
import tempfile
import unittest

from capture_software_evidence import capture


class CaptureTest(unittest.TestCase):
    def test_untracked_source_changes_and_private_inputs(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            subprocess.run(["git", "init", "-q"], cwd=root, check=True)
            subprocess.run(["git", "-c", "user.name=Test", "-c", "user.email=test@example.invalid",
                            "commit", "--allow-empty", "-qm", "fixture"], cwd=root, check=True)
            (root / "internal").mkdir()
            source = root / "internal" / "sample.go"
            source.write_text("package sample\n")
            (root / ".env").write_text("PRIVATE_FIXTURE=do-not-copy\n")
            report = root / "report.json"
            report.write_text("{}")
            destination = root / "inventory.json"
            first = capture(root, [report], destination)
            self.assertEqual(first, capture(root, [report], destination))
            self.assertEqual([row["path"] for row in first["sources"]], ["internal/sample.go"])
            self.assertNotIn("do-not-copy", json.dumps(first))
            source.write_text("package renamed\n")
            self.assertNotEqual(first["sha256"], capture(root, [report], destination)["sha256"])


if __name__ == "__main__":
    unittest.main()
