# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
"""Actual C handler and Python client; no device transport or RF claim."""
import ctypes
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

PACKAGE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(PACKAGE / "tools"))
import adhr_usb_bench as bench


class HandlerSmokeTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory()
        lib = Path(cls.temp.name) / "handler.so"
        args = ["cc", "-shared", "-fPIC", "-std=c11", "-Wall", "-Wextra", "-Werror"]
        for version, name in (("10.4", "adhr_mode"), ("10.5", "adhr_l2_envelope"),
                              ("10.7", "adhr_usb_bench")):
            application = PACKAGE.parent / f"opengd77-mvp{version}/overlay/MDUV380_firmware/application"
            args.extend(["-I" + str(application / "include"), str(application / f"source/adhr/{name}.c")])
        subprocess.run(args + [str(PACKAGE / "host/usb_host_bridge.c"), "-o", str(lib)], check=True)
        cls.lib = ctypes.CDLL(str(lib))
        cls.lib.host_exchange.argtypes = [ctypes.c_char_p, ctypes.c_size_t, ctypes.c_void_p, ctypes.c_size_t]
        cls.lib.host_exchange.restype = ctypes.c_size_t

    @classmethod
    def tearDownClass(cls):
        cls.temp.cleanup()

    def exchange(self, command, payload=b""):
        wire = bench.encode_request(command, payload)
        output = ctypes.create_string_buffer(39)
        count = self.lib.host_exchange(wire, len(wire), output, len(output))
        return bench.decode_response(output.raw[:count], command)

    def test_smoke_and_restore_both_modes(self):
        for mode in bench.MODES:
            with self.subTest(mode=mode):
                self.lib.adhrModeInitialize()
                self.lib.adhrL2Initialize()
                self.exchange(3, bytes([bench.MODES[mode]])).require_ok()
                result = bench.smoke(self)
                self.assertEqual(result["smoke"], "pass")
                self.assertEqual(result["restored"]["mode"], mode)
                self.assertEqual(result["restored"]["pending_count"], 0)

    def test_full_queue_mode_boundary_and_wrong_mode(self):
        self.lib.adhrModeInitialize()
        self.lib.adhrL2Initialize()
        self.exchange(3, b"\x02").require_ok()
        for _ in range(4):
            self.assertTrue(bench.describe_response(self.exchange(4, bench.GOLDEN_WIRE))["event_queued"])
        with self.assertRaisesRegex(RuntimeError, "pending_count"):
            bench.smoke(self)
        self.exchange(3, b"\x01").require_ok()
        self.assertEqual(self.exchange(5).status, 7)
        self.assertFalse(bench.describe_response(self.exchange(4, bench.GOLDEN_WIRE))["consumed"])
        self.assertEqual(bench.smoke(self)["restored"]["mode"], "fm")


if __name__ == "__main__":
    unittest.main()
