import unittest
from pathlib import Path
import tempfile

from local_data import LISTENING_PACK, require_local
from compare_codecs import adapters
from opus_overhead import BASE, PACK, bound, check_baseline, generate, pinned
from opus_repacketize import Repacketizer


class OverheadTest(unittest.TestCase):
    def test_exact_arithmetic_and_ideal_monotonicity(self):
        a=bound([24,24],80,19200,2)
        self.assertEqual(a["peak_protected_bits"],576)
        self.assertEqual(a["peak_reserved_cycle_ms"],68)
        self.assertTrue(a["arithmetic_peak_fits_not_service"])
        b=bound([120],160,19200,2,0)
        self.assertEqual(b["peak_reserved_cycle_ms"],164)
        self.assertFalse(b["arithmetic_peak_fits_not_service"])
        self.assertEqual(b["group_deadline_misses"],1)
        self.assertLess(bound([120],160,19200,2,0,0,0)["peak_reserved_cycle_ms"],164)
        self.assertIsNone(bound([10],20,19200,5)["peak_reservation_min_bps"])

    def test_invalid_bounds(self):
        for sizes,period,rate,opp in (([],80,19200,2),([True],80,19200,2),([0],80,19200,2),
                                      ([1]*3005,80,19200,2),([10],0,19200,2),([10],80,0,2),([10],80,19200,6)):
            with self.subTest(sizes=len(sizes),period=period,rate=rate,opp=opp), self.assertRaises(ValueError):
                bound(sizes,period,rate,opp)

    def test_baseline_mismatch_rejected(self):
        with self.assertRaises(ValueError):
            check_baseline(bound([24],80,19200,2),{"envelope":{
                "payload":{"max_bytes":25,"total_bytes":24,"total_protected_bits":576},
                "reserved_cycle_ms":68,"peak_reservation_min_bps":16000}})

    def test_provenance_fail_closed(self):
        with self.assertRaises(ValueError):
            pinned(PACK/"traces/opus-6000.json","0"*64)
        with self.assertRaises(ValueError):generate(Path("/tmp/not-an-overhead-output"))

    def test_full_native_speech_determinism_and_baseline(self):
        require_local(PACK)
        BASE.mkdir(parents=True,exist_ok=True)
        with tempfile.TemporaryDirectory(prefix="test-",dir=BASE) as tmp:
            root=Path(tmp)
            first,second=root/"a",root/"b"
            a,b=generate(first),generate(second)
            self.assertEqual(a,b)
            self.assertEqual((a["rows"],a["streams"],a["baseline_checks"]),(1440,16,288))
            for name in ("report.json","packing.json","manifest.json"):
                self.assertEqual((first/name).read_bytes(),(second/name).read_bytes())
            with self.assertRaises(ValueError):generate(first)


class NativePackingTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        require_local(LISTENING_PACK)
        _,_,cls.opus,_=adapters()
        cls.native=Repacketizer(cls.opus)
        cls.packets,_=cls.opus.encode(bytes(16000),False,6000)

    def test_native_identity_and_160ms_split(self):
        original,_=self.opus.decode_received(self.packets,False)
        for group in (1,2,4,8):
            chunks=self.native.group(self.packets,group)
            self.assertEqual(self.native.decode([p for c in chunks for p in c]),original)
            self.assertEqual(len(chunks),(len(self.packets)+group-1)//group)
            if group==8:self.assertGreaterEqual(len(chunks[0]),2)
        self.assertLessEqual(self.native.bodies(self.packets[0]),len(self.packets[0]))

    def test_malformed_and_bounds(self):
        for packets,group in (([],1),([b""],1),(self.packets,True),(self.packets,6)):
            with self.assertRaises(ValueError):self.native.group(packets,group)
        with self.assertRaises(ValueError):self.native.join(self.packets[:7])
        with self.assertRaises(ValueError):self.native.decode([bytes(10000)])
        with self.assertRaises(ValueError):self.native.decode([])

    def test_configuration_boundary_splits(self):
        from unittest.mock import patch
        # Isolate the split policy; real-packet validity is tested natively above.
        with patch.object(self.opus,"check_packet"), patch.object(self.native,"join",side_effect=lambda p:b"".join(p)):
            self.assertEqual(self.native.group([b"\x08a",b"\x08b",b"\x98c"],4),
                             [[b"\x08a\x08b",b"\x98c"]])


if __name__=="__main__":unittest.main()
