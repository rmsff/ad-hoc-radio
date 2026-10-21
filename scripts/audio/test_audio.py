"""Required tests use only stdlib and synthetic bytes, never claim codec quality."""
import io
import json
from pathlib import Path
import tempfile
import unittest
import wave

from common import (PACKET_SAMPLES, digest, erase_packets, erasure_mask, load_config,
                    pcm_stats, read_pcm, restore_timeline, write_wav)
from generate import bound_path


class PacketTests(unittest.TestCase):
    def test_mode_sizes_and_actual_packet_erasure(self):
        for mode, size in ((1600, 16), (3200, 32)):
            packets = [bytes([i])*size for i in range(4)]
            received = erase_packets(b"".join(packets), mode, [False, True, True, False])
            self.assertEqual(received, packets[0]+packets[3])
            self.assertEqual(len(received), 2*size)

    def test_timeline_restores_silence_not_clean_decoded_lost_audio(self):
        size = PACKET_SAMPLES*2
        output = restore_timeline(b"a"*size+b"b"*size, [False, True, True, False])
        self.assertEqual(output, b"a"*size+bytes(size*2)+b"b"*size)
        self.assertEqual(restore_timeline(b"", [True]*4), bytes(4*size))

    def test_mask_reproducible_nested_loss_and_burst(self):
        a = erasure_mask(400, "loss05", 42, "ru01")
        b = erasure_mask(400, "loss20", 42, "ru01")
        self.assertEqual(a, erasure_mask(400, "loss05", 42, "ru01"))
        self.assertNotEqual(a, erasure_mask(400, "loss05", 43, "ru01"))
        self.assertTrue(0 < sum(a) < sum(b) < 400)
        self.assertTrue(all(not x or y for x, y in zip(a, b)))
        burst = erasure_mask(400, "burst3", 42, "ru01")
        indices = [i for i, v in enumerate(burst) if v]
        self.assertEqual(indices, list(range(indices[0], indices[0]+3)))
        self.assertEqual(erasure_mask(1, "burst3", 42, "ru01"), [True])
        self.assertEqual(erasure_mask(5, "clean", 42, "ru01"), [False]*5)

    def test_malformed_packets_and_decoder_lengths_rejected(self):
        for mode, raw, mask in ((1600,b"short",[False]), (3200,bytes(31),[False]),
                                (700,bytes(16),[False]), (1600,bytes(16),[1]), (1600,b"",[])):
            with self.assertRaises(ValueError):
                erase_packets(raw, mode, mask)
        with self.assertRaises(ValueError):
            restore_timeline(bytes(1278), [False])
        for count, condition, seed in ((0,"clean",1),(751,"clean",1),(1,"unknown",1),(1,"clean",-1)):
            with self.assertRaises(ValueError):
                erasure_mask(count, condition, seed, "sample")


class InputTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def wav(self, rate=8000, channels=1, width=2, frames=8000):
        path = self.root / "input.wav"
        with wave.open(str(path), "wb") as w:
            w.setparams((channels,width,rate,0,"NONE","not compressed"))
            w.writeframes(bytes(frames*channels*width))
        return path

    def test_valid_and_invalid_pcm(self):
        path = self.wav()
        pcm, rate = read_pcm(path)
        self.assertEqual((len(pcm),rate),(16000,8000))
        for kwargs in ({"rate":16000},{"channels":2},{"width":1},{"frames":7999},{"frames":480001}):
            with self.assertRaises(ValueError):
                read_pcm(self.wav(**kwargs))
        path = self.wav()
        path.write_bytes(path.read_bytes()[:-2])
        with self.assertRaises(ValueError):
            read_pcm(path)

    def test_symlink_input_and_provenance_drift(self):
        original = self.wav()
        link = self.root / "link.wav"
        link.symlink_to(original)
        with self.assertRaises(ValueError):
            read_pcm(link)
        ref = {"path":"input.wav", **digest(original)}
        self.assertEqual(bound_path(self.root, ref), original)
        original.write_bytes(b"changed")
        with self.assertRaises(ValueError):
            bound_path(self.root, ref)
        with self.assertRaises(ValueError):
            bound_path(self.root, {**ref,"path":"../escape.wav"})

    def test_wave_roundtrip_and_stats(self):
        pcm = b"\xff\x7f\x00\x80"*4000
        out = self.root / "test.wav"
        write_wav(out,pcm)
        self.assertEqual(read_pcm(out)[0],pcm)
        s = pcm_stats(pcm)
        self.assertEqual(s["samples"],8000)
        self.assertEqual(s["clipped_samples"],8000)
        self.assertEqual(s["peak_abs"],32768)

    def test_config_duplicate_id_seed_and_cadence(self):
        cfg = json.loads((Path(__file__).resolve().parents[2]/"configs/audio/listening-20260906.json").read_text())
        path = self.root / "config.json"
        for edit in (lambda v:v.update(packet_ms=100),lambda v:v.update(seed=True),
                     lambda v:v["samples"][0].update(id="../bad"),
                     lambda v:v["samples"].append(v["samples"][0])):
            value = json.loads(json.dumps(cfg))
            edit(value)
            path.write_text(json.dumps(value))
            with self.assertRaises(ValueError):
                load_config(path)


if __name__ == "__main__":
    unittest.main()
