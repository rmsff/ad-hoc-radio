# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
from __future__ import annotations

import importlib.util
from pathlib import Path
import sys
import unittest


TOOL_PATH = Path(__file__).parents[1] / "tools" / "adhr_usb_bench.py"
SPEC = importlib.util.spec_from_file_location("adhr_usb_bench", TOOL_PATH)
assert SPEC is not None and SPEC.loader is not None
bench = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = bench
SPEC.loader.exec_module(bench)


class SmokeTransport:
    def __init__(self, original_mode: int = 1, sequence: int = 9):
        next_sequence = (sequence + 1) & 0xFFFFFFFF

        def status(mode: int, seq: int) -> bytearray:
            payload = bytearray(32)
            payload[0] = mode
            payload[12:16] = seq.to_bytes(4, "big")
            return payload

        event = bytearray(34)
        event[:4] = bytes((1, 1, 0, 22))
        event[4:8] = next_sequence.to_bytes(4, "big")
        event[8:12] = (123).to_bytes(4, "big")
        event[12:] = bench.GOLDEN_WIRE[5:]
        self.payloads = {
            "identity": bytearray((1, 1, 27, 34, 4, 0x9F, 0, 0)),
            "before": status(original_mode, sequence),
            "mode": bytearray((2,)),
            "active": status(2, sequence),
            "injection": bytearray((1, 1, 0, 0)) + next_sequence.to_bytes(4, "big"),
            "event": event,
            "after_event": status(2, next_sequence),
            "restore": bytearray((original_mode,)),
            "restored": status(original_mode, next_sequence),
        }
        self.errors: dict[str, Exception] = {}
        self.calls: list[tuple[int, bytes]] = []
        self.stages: list[str] = []
        self.mode_calls = 0
        self.injected = False

    def exchange(self, command: int, payload: bytes = b""):
        self.calls.append((command, payload))
        if command == bench.COMMANDS["identity"]:
            stage = "identity"
        elif command == bench.COMMANDS["set-mode"]:
            self.mode_calls += 1
            stage = "mode" if self.mode_calls == 1 else "restore"
        elif command == bench.COMMANDS["status"]:
            if self.mode_calls == 0:
                stage = "before"
            elif self.mode_calls == 2:
                stage = "restored"
            else:
                stage = "after_event" if self.injected else "active"
        elif command == bench.COMMANDS["inject"]:
            self.injected = True
            stage = "injection"
        elif command == bench.COMMANDS["pop"]:
            stage = "event"
        else:
            raise AssertionError(f"unexpected command {command}")
        self.stages.append(stage)
        if stage in self.errors:
            raise self.errors[stage]
        data = self.payloads[stage]
        wire = bytes((0x41, 1, command, 0, len(data))) + data
        return bench.decode_response(wire, command)


class SmokeTest(unittest.TestCase):
    def test_pass_requires_matching_frame_and_restored_mode(self) -> None:
        for mode in (1, 2):
            for sequence in (9, 0xFFFFFFFF):
                with self.subTest(mode=mode, sequence=sequence):
                    transport = SmokeTransport(mode, sequence)
                    result = bench.smoke(transport)
                    self.assertEqual(result["smoke"], "pass")
                    self.assertEqual(result["original_mode"], bench.MODE_NAMES[mode])
                    self.assertEqual(result["restored"]["mode"], bench.MODE_NAMES[mode])
                    events = [step for step in result["steps"] if step["command"] == 5]
                    self.assertEqual(events[0]["payload_hex"], bench.GOLDEN_WIRE[5:].hex())
                    self.assertEqual(events[0]["sequence"], (sequence + 1) & 0xFFFFFFFF)
                    self.assertEqual(transport.calls[-2:], [(3, bytes((mode,))), (2, b"")])

    def test_rejects_identity_drift_before_mode_control(self) -> None:
        for offset, value in ((0, 2), (1, 2), (2, 36), (3, 35), (4, 8),
                              (5, 0x1F), (5, 0x9D), (6, 1), (7, 1)):
            with self.subTest(offset=offset, value=value):
                transport = SmokeTransport()
                transport.payloads["identity"][offset] = value
                with self.assertRaises(RuntimeError):
                    bench.smoke(transport)
                self.assertEqual(transport.mode_calls, 0)

    def test_rejects_tx_flags_at_every_status_boundary(self) -> None:
        for stage in ("before", "active", "after_event", "restored"):
            for offset in (3, 4):
                with self.subTest(stage=stage, offset=offset):
                    transport = SmokeTransport()
                    transport.payloads[stage][offset] = 1
                    with self.assertRaises(RuntimeError):
                        bench.smoke(transport)
                    self.assertEqual(transport.mode_calls, 0 if stage == "before" else 2)

    def test_rejects_wrong_mode_ack_or_status(self) -> None:
        for stage, value in (("before", 0), ("mode", 1), ("active", 1),
                             ("after_event", 1), ("restore", 2), ("restored", 2)):
            with self.subTest(stage=stage):
                transport = SmokeTransport()
                transport.payloads[stage][0] = value
                with self.assertRaises(RuntimeError):
                    bench.smoke(transport)
                self.assertEqual(transport.mode_calls, 0 if stage == "before" else 2)

    def test_rejects_pending_queue_without_injection(self) -> None:
        transport = SmokeTransport()
        transport.payloads["active"][5] = 1
        with self.assertRaises(RuntimeError):
            bench.smoke(transport)
        self.assertFalse(transport.injected)
        self.assertIn("restored", transport.stages)

    def test_rejects_injection_not_accepted_or_not_queued(self) -> None:
        for offset, value in ((0, 0), (1, 0), (2, 8), (2, 10), (2, 1)):
            with self.subTest(offset=offset, value=value):
                transport = SmokeTransport()
                transport.payloads["injection"][offset] = value
                with self.assertRaises(RuntimeError):
                    bench.smoke(transport)
                self.assertNotIn("event", transport.stages)
                self.assertIn("restored", transport.stages)

    def test_rejects_stale_injection_even_with_matching_event(self) -> None:
        transport = SmokeTransport()
        for stage in ("injection", "event"):
            transport.payloads[stage][4:8] = (9).to_bytes(4, "big")
        with self.assertRaises(RuntimeError):
            bench.smoke(transport)
        self.assertIn("restored", transport.stages)

    def test_rejects_wrong_event_metadata_payload_or_sequence(self) -> None:
        for offset, value in ((0, 2), (1, 2), (2, 8), (3, 0), (7, 9),
                              (7, 11), (12, 0), (33, 0)):
            with self.subTest(offset=offset, value=value):
                transport = SmokeTransport()
                transport.payloads["event"][offset] = value
                with self.assertRaises(RuntimeError):
                    bench.smoke(transport)
                self.assertIn("restored", transport.stages)

    def test_restores_mode_after_transport_failure(self) -> None:
        for stage in ("mode", "active", "injection", "event", "after_event"):
            with self.subTest(stage=stage):
                transport = SmokeTransport()
                transport.errors[stage] = TimeoutError(stage)
                with self.assertRaisesRegex(TimeoutError, stage):
                    bench.smoke(transport)
                self.assertIn("restored", transport.stages)
                self.assertEqual(transport.calls[-2:], [(3, b"\x01"), (2, b"")])

    def test_cannot_pass_when_restoration_fails(self) -> None:
        for stage in ("restore", "restored"):
            with self.subTest(stage=stage):
                transport = SmokeTransport()
                transport.errors[stage] = TimeoutError(stage)
                with self.assertRaisesRegex(TimeoutError, stage):
                    bench.smoke(transport)


class ProtocolTest(unittest.TestCase):
    def test_request_encoding(self) -> None:
        self.assertEqual(bench.encode_request(3, b"\x02"), b"A\x01\x03\x01\x02")

    def test_status_decoding(self) -> None:
        payload = bytearray(32)
        payload[0] = 2
        payload[3] = 0
        payload[5] = 1
        payload[8:12] = (7).to_bytes(4, "big")
        payload[12:16] = (9).to_bytes(4, "big")
        payload[20:24] = (4).to_bytes(4, "big")
        wire = bytes((0x41, 1, 2, 0, 32)) + payload
        result = bench.describe_response(bench.decode_response(wire, 2))
        self.assertEqual(result["mode"], "adhr")
        self.assertEqual(result["pending_count"], 1)
        self.assertEqual(result["transition_sequence"], 7)
        self.assertEqual(result["l2_sequence"], 9)
        self.assertEqual(result["accepted_count"], 4)
        self.assertFalse(result["logical_tx_allowed"])

    def test_event_decoding(self) -> None:
        payload = bytearray(34)
        payload[0:4] = bytes((1, 1, 0, 22))
        payload[4:8] = (3).to_bytes(4, "big")
        payload[8:12] = (0x01020304).to_bytes(4, "big")
        payload[12:34] = bench.GOLDEN_WIRE[5:]
        wire = bytes((0x41, 1, 5, 0, 34)) + payload
        result = bench.describe_response(bench.decode_response(wire, 5))
        self.assertEqual(result["event_kind"], 1)
        self.assertEqual(result["sequence"], 3)
        self.assertEqual(result["observed_tick"], 0x01020304)
        self.assertEqual(result["payload_hex"], bench.GOLDEN_WIRE[5:].hex())

    def test_rejects_length_and_command_mismatch(self) -> None:
        with self.assertRaisesRegex(ValueError, "payload length mismatch"):
            bench.decode_response(b"A\x01\x02\x00\x01")
        with self.assertRaisesRegex(ValueError, "does not match"):
            bench.decode_response(b"A\x01\x02\x00\x00", 1)


if __name__ == "__main__":
    unittest.main()
