"""Synthetic parser tests, not proof of native Lyra execution."""
from pathlib import Path
import json
import shutil
import tempfile
import unittest
import wave

from lyra_build import OUT
from lyra_probe import packets, padded_wav, read_pcm
from lyra_verify import verify


class TraceTests(unittest.TestCase):
    def test_real_bytes_round_trip_and_cadence(self):
        raw = bytes(range(16))
        self.assertEqual(bytes.fromhex("".join(packets(raw, 161))), raw)
        self.assertEqual(len(packets(raw, 320)), 2)
        self.assertEqual(len(packets(bytes(8), 160)), 1)

    def test_reject_wrong_lengths(self):
        for raw in (b"", bytes(7), bytes(9), bytes(16)):
            with self.subTest(size=len(raw)), self.assertRaises(ValueError):
                packets(raw, 160)

    def test_reject_invalid_counts(self):
        for count in (0, -1, True, 1.5, "160", None):
            with self.subTest(count=count), self.assertRaises(ValueError):
                packets(bytes(8), count)

    def test_padding_preserves_input(self):
        OUT.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=OUT) as temporary:
            source, target = Path(temporary) / "in.wav", Path(temporary) / "out.wav"
            pcm = b"\x01\x02" * 161
            with wave.open(str(source), "wb") as wav:
                wav.setparams((1, 2, 8000, 0, "NONE", "not compressed"))
                wav.writeframes(pcm)
            self.assertEqual(padded_wav(source, target), (161, 159))
            self.assertEqual(read_pcm(target), (320, pcm + bytes(318)))

    def test_reject_wrong_sample_rate(self):
        OUT.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(dir=OUT) as temporary:
            source = Path(temporary) / "bad.wav"
            with wave.open(str(source), "wb") as wav:
                wav.setparams((1, 2, 16000, 0, "NONE", "not compressed"))
                wav.writeframes(bytes(320))
            with self.assertRaises(ValueError):
                read_pcm(source)


@unittest.skipUnless((OUT / "final-2/packet-trace.json").exists(), "native final runs absent")
class NativeReceiptTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(dir=OUT)
        self.addCleanup(self.temporary.cleanup)
        self.copy = Path(self.temporary.name) / "copy"
        shutil.copytree(OUT / "final-1", self.copy)

    def test_actual_runs(self):
        self.assertTrue(verify(self.copy, OUT / "final-2")["verified"])

    def test_metadata_mutation(self):
        path = self.copy / "packet-trace.json"
        value = json.loads(path.read_text())
        value["lookahead_us"] = 0
        path.write_text(json.dumps(value))
        with self.assertRaisesRegex(ValueError, "metadata"):
            verify(self.copy, OUT / "final-2")

    def test_packet_mutation(self):
        path = self.copy / "ru01-padded8k.lyra"
        raw = bytearray(path.read_bytes())
        raw[0] ^= 1
        path.write_bytes(raw)
        with self.assertRaisesRegex(ValueError, "packet bytes"):
            verify(self.copy, OUT / "final-2")

    def test_model_mutation(self):
        (self.copy / "model_coeffs/lyra_config.binarypb").write_bytes(b"\0\0")
        with self.assertRaisesRegex(ValueError, "binary/model"):
            verify(self.copy, OUT / "final-2")


if __name__ == "__main__":
    unittest.main()
