"""Trace boundary tests run without libopus; native tests never fake replay."""
import copy
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

from common import digest
import export_opus_trace as trace
from opus_generate import ROOT, SETTINGS


def save(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value), encoding="utf-8")


class TraceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "pack"
        self.output = Path(self.temp.name) / "trace.json"
        self.original = {"id": "ru01", "speaker": "Speaker", "author": "Author",
                         "source_page": "https://example.org/recording",
                         "url": "https://example.org/audio.ogg",
                         "text_url": "https://example.org/text", "license": trace.LICENSE,
                         "license_url": "https://creativecommons.org/licenses/by-sa/4.0/"}
        save(self.root / "evidence/source-config.json", {"samples": [self.original]})
        save(self.root / "evidence/source-manifest.json", {"files": [{
            "path": "evidence/source-config.json", **digest(self.root / "evidence/source-config.json")}]})
        self.cfg = {"samples": ["ru01"], "settings": SETTINGS, "library": {"sha256": "a" * 64},
                    "source_manifest_sha256": digest(self.root / "evidence/source-manifest.json")["sha256"]}
        save(self.root / "evidence/config.json", self.cfg)
        self.attribution = ("Speaker; Author; https://example.org/recording\n"
                            "https://creativecommons.org/licenses/by-sa/4.0/\n"
                            "Changes: Opus conversion. No endorsement.\n")
        (self.root / "ATTRIBUTION.txt").write_text(self.attribution, encoding="utf-8")
        self.data = {"packets": ["00", "0102", "030405"] * 17,
                     "meta": {"input_samples": 8000, "lookahead_samples": 52,
                              "packet_lengths": [1, 2, 3] * 17}}
        save(self.root / "evidence/ru01-fecoff.json", self.data)
        self.manifest = {"version": 1, "settings": SETTINGS, "library": self.cfg["library"],
                         "source_manifest_sha256": self.cfg["source_manifest_sha256"],
                         "encodings": [{"sample_id": "ru01", "encoder": "fecoff",
                                        "evidence": "evidence/ru01-fecoff.json", "stats": {"packets": 51}}]}
        self.seal()
        self.verifier = patch.object(trace.opus_verify, "verify", side_effect=lambda root: {
            "verified": True, "manifest": digest(root / "manifest.json")}).start()
        self.addCleanup(patch.stopall)

    def seal(self):
        self.manifest["files"] = [{"path": str(p.relative_to(self.root)), **digest(p)}
                                  for p in sorted(self.root.rglob("*"))
                                  if p.is_file() and p.name != "manifest.json"]
        save(self.root / "manifest.json", self.manifest)

    def rejected(self, message):
        with self.assertRaisesRegex(ValueError, message):
            trace.export(self.root, self.output)
        self.assertFalse(self.output.exists())

    def test_exact_schema_packets_receipts_and_attribution(self):
        result = trace.export(self.root, self.output)
        self.assertEqual(set(result), {"version", "codec", "source_manifest_sha256", "library_sha256",
                                      "packet_ms", "lookahead_us", "license", "attribution", "samples"})
        self.assertEqual(result["samples"], [{"id": "ru01", "input_samples": 8000,
                                              "packets_hex": self.data["packets"]}])
        self.assertEqual(result["source_manifest_sha256"], digest(self.root / "manifest.json")["sha256"])
        self.assertEqual(result["library_sha256"], "a" * 64)
        self.assertEqual(result["codec"], "opus-8000-nb-20ms-cvbr")
        self.assertEqual((result["version"], result["packet_ms"], result["lookahead_us"]), (1, 20, 6500))
        self.assertEqual(result["license"], "CC-BY-SA-4.0")
        self.assertTrue(result["attribution"].startswith(self.attribution))
        for key in ("url", "text_url", "source_page", "speaker", "author", "license_url"):
            self.assertIn(self.original[key], result["attribution"])
        self.assertEqual(json.loads(self.output.read_text()), result)
        self.verifier.assert_called_once_with(self.root.resolve())

    def test_missing_fecoff_never_substitutes_fecon(self):
        self.manifest["encodings"][0]["encoder"] = "fecon"
        self.seal()
        self.rejected("missing FEC-off")
        self.verifier.assert_not_called()

    def test_duplicate_and_unsafe_ids(self):
        for ids in (["ru01", "ru01"], [], ["../ru01"], [True], ["ru01"] * 9):
            with self.subTest(ids=ids):
                self.cfg["samples"] = ids
                save(self.root / "evidence/config.json", self.cfg)
                self.seal()
                self.rejected("sample")

    def test_altered_source_and_provenance(self):
        (self.root / "evidence/source-manifest.json").write_text("{}")
        self.rejected("source artifact mismatch")
        self.seal()
        self.rejected("source manifest changed")
        self.verifier.assert_not_called()

    def test_attribution_must_match_original_receipt(self):
        save(self.root / "evidence/source-config.json", {"samples": []})
        self.seal()
        self.rejected("source attribution provenance")

    def test_missing_attribution(self):
        (self.root / "ATTRIBUTION.txt").write_text("Opus. No endorsement.")
        self.seal()
        self.rejected("missing attribution")

    def test_altered_packets(self):
        self.data["packets"][0] = "ff"
        save(self.root / "evidence/ru01-fecoff.json", self.data)
        self.rejected("source artifact mismatch")

    def test_bad_hex_and_packet_size(self):
        for packet in (None, True, 7, "", "0", "zz", "00  ", "00" * 1276):
            with self.subTest(packet=str(packet)[:20]):
                self.data["packets"][0] = packet
                save(self.root / "evidence/ru01-fecoff.json", self.data)
                self.seal()
                self.rejected("invalid packet")

    def test_packet_count_and_exact_flush_tail(self):
        for count in (0, 1, 50, 52, 3005):
            with self.subTest(count=count):
                self.data["packets"] = ["00"] * count
                save(self.root / "evidence/ru01-fecoff.json", self.data)
                self.seal()
                self.rejected("packet count")

    def test_maximum_duration_and_packet_length_preserved(self):
        self.data["packets"] = ["00"] * 3000 + ["ff" * 1275]
        self.data["meta"]["input_samples"] = 480000
        self.data["meta"]["packet_lengths"] = [1] * 3000 + [1275]
        self.manifest["encodings"][0]["stats"]["packets"] = 3001
        save(self.root / "evidence/ru01-fecoff.json", self.data)
        self.seal()
        result = trace.export(self.root, self.output)
        self.assertEqual(result["samples"][0]["packets_hex"], self.data["packets"])

    def test_lengths_and_declared_count_cannot_hide_packets(self):
        self.data["meta"]["packet_lengths"][0] = 2
        save(self.root / "evidence/ru01-fecoff.json", self.data)
        self.seal()
        self.rejected("packet lengths mismatch")
        self.data["meta"]["packet_lengths"][0] = 1
        self.manifest["encodings"][0]["stats"]["packets"] = 50
        save(self.root / "evidence/ru01-fecoff.json", self.data)
        self.seal()
        self.rejected("declared packet count")

    def test_duration_and_lookahead(self):
        for count in (True, 8000.0, 7999, 480001):
            self.data["meta"]["input_samples"] = count
            save(self.root / "evidence/ru01-fecoff.json", self.data)
            self.seal()
            self.rejected("input duration")
        self.data["meta"]["input_samples"] = 8000
        self.data["meta"]["lookahead_samples"] = 53
        save(self.root / "evidence/ru01-fecoff.json", self.data)
        self.seal()
        self.rejected("lookahead")

    def test_duplicate_json_key_and_nonfinite_and_oversize(self):
        for raw in ('{"version":1,"version":1}', '{"version":NaN}', "[]",
                    "{}" + " " * trace.MAX_JSON_BYTES):
            (self.root / "manifest.json").write_text(raw)
            self.rejected("duplicate JSON|invalid JSON|JSON object|oversize JSON")

    def test_verification_failure_never_writes(self):
        self.verifier.side_effect = ValueError("encode replay mismatch")
        self.rejected("encode replay mismatch")

    def test_verification_cannot_be_skipped_or_mismatched(self):
        for receipt in ({"verified": False}, {"verified": True, "manifest": {"sha256": "0" * 64}}):
            self.verifier.side_effect = None
            self.verifier.return_value = receipt
            self.rejected("verification receipt mismatch")

    def test_changes_during_verification_are_rejected(self):
        def change_source(root):
            receipt = {"verified": True, "manifest": digest(root / "manifest.json")}
            self.data["packets"][0] = "ff"
            save(root / "evidence/ru01-fecoff.json", self.data)
            self.seal()
            return receipt

        self.verifier.side_effect = change_source
        self.rejected("source changed during verification")

    def test_exclusive_create_handles_output_race(self):
        def create_output(root):
            self.output.write_bytes(b"another writer")
            return {"verified": True, "manifest": digest(root / "manifest.json")}

        self.verifier.side_effect = create_output
        with self.assertRaises(FileExistsError):
            trace.export(self.root, self.output)
        self.assertEqual(self.output.read_bytes(), b"another writer")

    def test_no_overwrite_or_dangling_symlink(self):
        self.output.write_bytes(b"existing receipt")
        with self.assertRaises(FileExistsError):
            trace.export(self.root, self.output)
        self.assertEqual(self.output.read_bytes(), b"existing receipt")
        self.output.unlink()
        self.output.symlink_to(self.output.parent / "missing.json")
        with self.assertRaises(FileExistsError):
            trace.export(self.root, self.output)
        self.verifier.assert_not_called()

    def test_no_output_inside_source_pack(self):
        with self.assertRaisesRegex(ValueError, "must not alter"):
            trace.export(self.root, self.root / "trace.json")

    def test_deterministic_fresh_outputs(self):
        trace.export(self.root, self.output)
        second = self.output.with_name("second.json")
        trace.export(self.root, second)
        self.assertEqual(self.output.read_bytes(), second.read_bytes())

    def test_duplicate_encoding_and_path_escape(self):
        self.manifest["encodings"].append(copy.deepcopy(self.manifest["encodings"][0]))
        self.seal()
        self.rejected("duplicate encoding")
        self.manifest["encodings"].pop()
        self.manifest["encodings"][0]["evidence"] = "../outside.json"
        self.seal()
        self.rejected("unsafe artifact path")


class TraceNativeTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cfg = json.loads((ROOT / "configs/audio/opus-listening-20260906.json").read_text())
        cls.pack = ROOT / ".artifacts/opus-listening-20260906/final-r1"
        if not Path(cfg["library"]["path"]).is_file() or not cls.pack.is_dir():
            raise unittest.SkipTest("optional pinned libopus/listening pack is absent")

    def test_real_export_replay_and_preserved_packets(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "trace.json"
            result = trace.export(self.pack, output)
            self.assertEqual(result["source_manifest_sha256"],
                             "38d90039f01044dbfb9319424f36dac4646540730fe3a2fc8e0722d8215e9b65")
            self.assertEqual([len(s["packets_hex"]) for s in result["samples"]], [774, 1634])
            for sample in result["samples"]:
                source = json.loads((self.pack / f"evidence/{sample['id']}-fecoff-encoded.json").read_text())
                self.assertEqual(sample["packets_hex"], source["packets"])
                self.assertEqual(sample["input_samples"], source["meta"]["input_samples"])
            second = output.with_name("second.json")
            trace.export(self.pack, second)
            self.assertEqual(output.read_bytes(), second.read_bytes())

    def test_real_altered_pcm_fails_without_output(self):
        with tempfile.TemporaryDirectory() as directory:
            pack, output = Path(directory) / "pack", Path(directory) / "trace.json"
            shutil.copytree(self.pack, pack)
            wav = pack / "wav/ru01-input8k.wav"
            raw = wav.read_bytes()
            wav.write_bytes(raw[:-1] + bytes([raw[-1] ^ 1]))
            with self.assertRaisesRegex(ValueError, "hash mismatch"):
                trace.export(pack, output)
            self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
