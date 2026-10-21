"""Optional native Opus checks; synthetic/unit results are not listening scores."""
import hashlib
from pathlib import Path
import tempfile
import unittest

from common import read_pcm, save
from local_data import LISTENING_PACK, require_local
from opus_codec import FRAME, Opus, align, erase, loss_mask
from opus_generate import ROOT, child, config, generate, source_inputs
from opus_verify import packets, verify

CONFIG = ROOT / "configs/audio/opus-listening-20260906.json"


class OpusPureTests(unittest.TestCase):
    def test_mask_determinism_and_nested_losses(self):
        masks = [loss_mask(1000, c, 20260906, "ru01") for c in ("loss05", "loss20", "loss30")]
        self.assertEqual(masks[0], loss_mask(1000, "loss05", 20260906, "ru01"))
        self.assertTrue(all(not a or b for a, b in zip(masks[0], masks[1])))
        self.assertTrue(all(not a or b for a, b in zip(masks[1], masks[2])))
        self.assertEqual(sum(loss_mask(100, "clean", 0, "s")), 0)

    def test_burst_is_twelve_adjacent_frames(self):
        mask = loss_mask(100, "burst240", 9, "sample")
        positions = [i for i, lost in enumerate(mask) if lost]
        self.assertEqual(positions, list(range(positions[0], positions[0]+12)))
        self.assertEqual(loss_mask(3, "burst240", 9, "sample"), [True]*3)

    def test_loss_removes_payload(self):
        self.assertEqual(erase([b"secret", b"next"], [True, False]), [None, b"next"])
        for mask in ([True], [1, 0], []):
            with self.assertRaises(ValueError):
                erase([b"a", b"b"], mask)

    def test_invalid_bounds(self):
        for count, seed in ((0, 0), (3005, 0), (True, 0), (2, -1), (2, True)):
            with self.assertRaises(ValueError):
                loss_mask(count, "clean", seed, "s")
        with self.assertRaises(ValueError):
            loss_mask(10, "made-up", 0, "s")
        for values in ([], [""], ["00"*1276], [42], ["xx"]):
            with self.assertRaises(ValueError):
                packets(values)

    def test_path_and_alignment_guards(self):
        for name in ("../escape", "/tmp/escape"):
            with self.assertRaises(ValueError):
                child(ROOT, name)
        self.assertEqual(align(b"00112233", {"lookahead_samples": 1, "input_samples": 2}), b"1122")
        with self.assertRaises(ValueError):
            align(b"00", {"lookahead_samples": 1, "input_samples": 2})

    def test_config_never_ignores_settings(self):
        cfg = config(CONFIG)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "config.json"
            cfg["settings"]["packet_ms"] = 80
            save(path, cfg)
            with self.assertRaises(ValueError):
                config(path)


class OpusNativeTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cfg = config(CONFIG)
        if not Path(cls.cfg["library"]["path"]).is_file():
            raise unittest.SkipTest("optional pinned host libopus is not installed")
        require_local(LISTENING_PACK)
        cls.opus = Opus(cls.cfg["library"]["path"], cls.cfg["library"]["sha256"])
        cls.source, _ = source_inputs(cls.cfg)
        cls.pcm, _ = read_pcm(cls.source / "wav/ru01-input8k.wav")
        cls.payload, cls.meta = cls.opus.encode(cls.pcm, True)
        cls.control, cls.control_meta = cls.opus.encode(cls.pcm, True, 24000)

    def test_library_pin_rejected(self):
        with self.assertRaises(ValueError):
            Opus(self.cfg["library"]["path"], "0"*64)

    def test_clean_delay_duration_and_no_fabricated_fec(self):
        decoded, actions = self.opus.decode_received(self.payload, False)
        decoded_fec, fec_actions = self.opus.decode_received(self.payload, True)
        self.assertEqual(decoded, decoded_fec)
        self.assertEqual(actions, fec_actions)
        self.assertEqual(len(align(decoded, self.meta)), len(self.pcm))
        self.assertEqual(set(actions), {"normal"})
        self.assertNotEqual(hashlib.sha256(align(decoded, self.meta)).digest(), hashlib.sha256(self.pcm).digest())
        self.assertEqual(sum(self.meta["lbrr"]), 0, "pinned 8k fixture changed; inspect evidence")

    def test_positive_fec_control_and_causal_fallback(self):
        # Test-only 24k NB isolates the FEC ABI, not the official 24k wideband demo.
        index = next(i-1 for i, flag in enumerate(self.control_meta["lbrr"]) if flag and i > 2)
        received = list(self.control)
        received[index] = None
        raw, actions = self.opus.decode_received(received, True)
        plc, plc_actions = self.opus.decode_received(received, False)
        self.assertEqual(actions[index], "fec")
        self.assertEqual(plc_actions[index], "plc")
        self.assertNotEqual(raw, plc)
        self.assertEqual(len(raw), len(received)*FRAME*2)
        received[index+1] = None
        received[-1] = None
        _, burst_actions = self.opus.decode_received(received, True)
        self.assertEqual(burst_actions[index], "plc", "must not reach across missing successor")
        self.assertEqual(burst_actions[-1], "plc", "cannot recover from nonexistent future")

    def test_all_lost_and_fec_absent(self):
        raw, actions = self.opus.decode_received([None]*10, True)
        self.assertEqual(len(raw), 10*FRAME*2)
        self.assertEqual(actions, ["plc"]*10)
        mask = loss_mask(len(self.payload), "loss30", 20260906, "ru01")
        received = erase(self.payload, mask)
        self.assertEqual(self.opus.decode_received(received, True), self.opus.decode_received(received, False))

    def test_invalid_native_arguments(self):
        for payload in (b"", b"a"*1276, "not bytes"):
            with self.assertRaises(ValueError):
                self.opus.check_packet(payload)
        for pcm in (b"", b"0"*16001, bytes(960002)):
            with self.assertRaises(ValueError):
                self.opus.encode(pcm, False)
        with self.assertRaises(ValueError):
            self.opus.encode(self.pcm, False, 12000)
        with self.assertRaises(ValueError):
            self.opus.decode_received([], True)

    def test_generate_replay_and_no_overwrite(self):
        base = ROOT / ".artifacts/opus-listening-20260906"
        base.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(prefix="unit-", dir=base) as directory:
            output = Path(directory) / "pack"
            generate(CONFIG, output)
            self.assertEqual(verify(output)["records"], 20)
            with self.assertRaises(FileExistsError):
                generate(CONFIG, output)
            wav = output / "wav/ru01-fecoff-clean-plc.wav"
            original = wav.read_bytes()
            wav.write_bytes(original[:-1] + bytes([original[-1] ^ 1]))
            with self.assertRaisesRegex(ValueError, "hash mismatch"):
                verify(output)


if __name__ == "__main__":
    unittest.main()
