#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
"""Dependency-free host client for the receive-only ADHR USB bench protocol."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import json
import os
from pathlib import Path
import select
import termios
import time
import tty


MAGIC = 0x41
VERSION = 1
REQUEST_HEADER_SIZE = 4
RESPONSE_HEADER_SIZE = 5
COMMANDS = {
    "identity": 1,
    "status": 2,
    "set-mode": 3,
    "inject": 4,
    "pop": 5,
    "lifecycle": 6,
    "reset": 7,
}
STATUSES = {
    0: "ok",
    1: "bad_magic",
    2: "bad_version",
    3: "bad_length",
    4: "bad_command",
    5: "bad_payload",
    6: "mode_rejected",
    7: "no_event",
    8: "tx_guard",
    9: "internal",
}
MODES = {"fm": 1, "adhr": 2}
MODE_NAMES = {value: name for name, value in MODES.items()}
LIFECYCLE_REASONS = {"timeout": 9, "reset": 11, "disconnect": 12}
GOLDEN_WIRE = bytes.fromhex(
    "a7011600fc011000020314cbf0b66eb710f7eb1020304050607080"
)


@dataclass(frozen=True)
class Response:
    command: int
    status: int
    payload: bytes

    @property
    def status_name(self) -> str:
        return STATUSES.get(self.status, f"unknown_{self.status}")

    def require_ok(self) -> "Response":
        if self.status != 0:
            raise RuntimeError(f"bench command {self.command} failed: {self.status_name}")
        return self


def encode_request(command: int, payload: bytes = b"") -> bytes:
    if not 0 <= command <= 0xFF:
        raise ValueError("command must fit in one byte")
    if len(payload) > 0xFF:
        raise ValueError("payload is too large")
    return bytes((MAGIC, VERSION, command, len(payload))) + payload


def decode_response(data: bytes, expected_command: int | None = None) -> Response:
    if len(data) < RESPONSE_HEADER_SIZE:
        raise ValueError("response is shorter than its header")
    magic, version, command, status, payload_length = data[:RESPONSE_HEADER_SIZE]
    if magic != MAGIC:
        raise ValueError(f"unexpected response magic 0x{magic:02x}")
    if version != VERSION:
        raise ValueError(f"unsupported response version {version}")
    if expected_command is not None and command != expected_command:
        raise ValueError(f"response command {command} does not match request {expected_command}")
    if len(data) != RESPONSE_HEADER_SIZE + payload_length:
        raise ValueError("response payload length mismatch")
    return Response(command=command, status=status, payload=data[RESPONSE_HEADER_SIZE:])


def _u32(payload: bytes, offset: int) -> int:
    return int.from_bytes(payload[offset : offset + 4], "big")


def describe_response(response: Response) -> dict[str, object]:
    result: dict[str, object] = {
        "command": response.command,
        "status": response.status_name,
    }
    if response.status != 0:
        return result
    payload = response.payload
    if response.command == COMMANDS["identity"]:
        if len(payload) != 8:
            raise ValueError("identity payload must be 8 bytes")
        result.update(
            protocol_version=payload[0],
            l2_version=payload[1],
            l2_wire_size=payload[2],
            event_wire_size=payload[3],
            queue_capacity=payload[4],
            capabilities=payload[5],
            rx_only=bool(payload[5] & 0x80),
        )
    elif response.command == COMMANDS["status"]:
        if len(payload) != 32:
            raise ValueError("status payload must be 32 bytes")
        result.update(
            mode=MODE_NAMES.get(payload[0], f"unknown_{payload[0]}"),
            last_reason=payload[1],
            last_error=payload[2],
            physical_tx_flags=payload[3],
            logical_tx_allowed=bool(payload[4]),
            pending_count=payload[5],
            transition_sequence=_u32(payload, 8),
            l2_sequence=_u32(payload, 12),
            l2_generation=_u32(payload, 16),
            accepted_count=_u32(payload, 20),
            rejected_count=_u32(payload, 24),
            pressure_count=_u32(payload, 28),
        )
    elif response.command == COMMANDS["set-mode"]:
        if len(payload) != 1:
            raise ValueError("set-mode payload must be 1 byte")
        result["mode"] = MODE_NAMES.get(payload[0], f"unknown_{payload[0]}")
    elif response.command == COMMANDS["inject"]:
        if len(payload) != 8:
            raise ValueError("inject payload must be 8 bytes")
        result.update(
            consumed=bool(payload[0]),
            event_queued=bool(payload[1]),
            reason=payload[2],
            sequence=_u32(payload, 4),
        )
    elif response.command == COMMANDS["pop"]:
        if len(payload) != 34:
            raise ValueError("event payload must be 34 bytes")
        result.update(
            event_version=payload[0],
            event_kind=payload[1],
            reason=payload[2],
            payload_length=payload[3],
            sequence=_u32(payload, 4),
            observed_tick=_u32(payload, 8),
            payload_hex=payload[12:34].hex(),
        )
    elif response.command == COMMANDS["lifecycle"]:
        if len(payload) != 4:
            raise ValueError("lifecycle payload must be 4 bytes")
        result.update(consumed=bool(payload[0]), event_queued=bool(payload[1]), reason=payload[2])
    elif response.command == COMMANDS["reset"] and payload:
        raise ValueError("reset response must not have a payload")
    return result


class SerialTransport:
    def __init__(self, port: Path, timeout: float = 2.0):
        self.port = port
        self.timeout = timeout
        self.fd: int | None = None
        self.saved_attributes: list[object] | None = None

    def __enter__(self) -> "SerialTransport":
        self.fd = os.open(self.port, os.O_RDWR | os.O_NOCTTY | os.O_NONBLOCK)
        self.saved_attributes = termios.tcgetattr(self.fd)
        tty.setraw(self.fd)
        termios.tcflush(self.fd, termios.TCIOFLUSH)
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        if self.fd is not None:
            if self.saved_attributes is not None:
                termios.tcsetattr(self.fd, termios.TCSANOW, self.saved_attributes)
            os.close(self.fd)
            self.fd = None

    def _read_exact(self, length: int, deadline: float) -> bytes:
        if self.fd is None:
            raise RuntimeError("serial transport is not open")
        data = bytearray()
        while len(data) < length:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError(f"timed out after {self.timeout}s reading {length} bytes")
            ready, _, _ = select.select([self.fd], [], [], remaining)
            if not ready:
                continue
            chunk = os.read(self.fd, length - len(data))
            if not chunk:
                raise EOFError("serial device closed while reading response")
            data.extend(chunk)
        return bytes(data)

    def exchange(self, command: int, payload: bytes = b"") -> Response:
        if self.fd is None:
            raise RuntimeError("serial transport is not open")
        request = encode_request(command, payload)
        termios.tcflush(self.fd, termios.TCIFLUSH)
        offset = 0
        while offset < len(request):
            offset += os.write(self.fd, request[offset:])
        deadline = time.monotonic() + self.timeout
        header = self._read_exact(RESPONSE_HEADER_SIZE, deadline)
        body = self._read_exact(header[4], deadline)
        return decode_response(header + body, expected_command=command)


def _require_smoke_fields(data: dict[str, object], expected: dict[str, object], stage: str) -> None:
    for field, value in expected.items():
        if data.get(field) != value:
            raise RuntimeError(f"smoke {stage}: {field}={data.get(field)!r}, expected {value!r}")


def _smoke_status(transport: SerialTransport, mode: str | None = None) -> dict[str, object]:
    data = describe_response(transport.exchange(COMMANDS["status"]).require_ok())
    if data["mode"] not in MODES:
        raise RuntimeError(f"smoke status: unsupported mode {data['mode']!r}")
    expected: dict[str, object] = {"physical_tx_flags": 0, "logical_tx_allowed": False}
    if mode is not None:
        expected["mode"] = mode
    _require_smoke_fields(data, expected, "status")
    return data


def smoke(transport: SerialTransport) -> dict[str, object]:
    identity = transport.exchange(COMMANDS["identity"]).require_ok()
    # Pin the v1 ABI, all advertised capabilities including RX-only, and reserved bytes.
    if identity.payload != bytes((1, 1, 27, 34, 4, 0x9F, 0, 0)):
        raise RuntimeError(f"smoke identity: unsupported RX-only identity {identity.payload.hex()}")
    before = _smoke_status(transport)
    original_mode_name = str(before["mode"])
    original_mode = MODES[original_mode_name]
    steps: list[dict[str, object]] = [describe_response(identity), before]
    try:
        selected = describe_response(
            transport.exchange(COMMANDS["set-mode"], bytes((MODES["adhr"],))).require_ok()
        )
        _require_smoke_fields(selected, {"mode": "adhr"}, "set-mode")
        active = _smoke_status(transport, "adhr")
        _require_smoke_fields(active, {"pending_count": 0}, "before injection")
        steps.extend((selected, active))

        injection = describe_response(transport.exchange(COMMANDS["inject"], GOLDEN_WIRE).require_ok())
        _require_smoke_fields(injection, {
            "consumed": True,
            "event_queued": True,
            "reason": 0,
            "sequence": (int(active["l2_sequence"]) + 1) & 0xFFFFFFFF,
        }, "injection")
        event = describe_response(transport.exchange(COMMANDS["pop"]).require_ok())
        _require_smoke_fields(event, {
            "event_version": 1,
            "event_kind": 1,
            "reason": 0,
            "payload_length": 22,
            "payload_hex": GOLDEN_WIRE[5:].hex(),
            "sequence": injection["sequence"],
        }, "event")
        steps.extend((injection, event, _smoke_status(transport, "adhr")))
    finally:
        restored = describe_response(
            transport.exchange(COMMANDS["set-mode"], bytes((original_mode,))).require_ok()
        )
        _require_smoke_fields(restored, {"mode": original_mode_name}, "restore")
        after = _smoke_status(transport, original_mode_name)
    return {"smoke": "pass", "original_mode": original_mode_name, "steps": steps, "restored": after}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--port", required=True, type=Path)
    parser.add_argument("--timeout", type=float, default=2.0)
    subparsers = parser.add_subparsers(dest="action", required=True)
    for action in ("identity", "status", "pop", "reset", "smoke"):
        subparsers.add_parser(action)
    set_mode = subparsers.add_parser("set-mode")
    set_mode.add_argument("mode", choices=tuple(MODES))
    inject = subparsers.add_parser("inject")
    inject.add_argument("--hex", default=GOLDEN_WIRE.hex())
    lifecycle = subparsers.add_parser("lifecycle")
    lifecycle.add_argument("reason", choices=tuple(LIFECYCLE_REASONS))
    args = parser.parse_args()

    with SerialTransport(args.port, args.timeout) as transport:
        if args.action == "smoke":
            result = smoke(transport)
        else:
            payload = b""
            if args.action == "set-mode":
                payload = bytes((MODES[args.mode],))
            elif args.action == "inject":
                payload = bytes.fromhex(args.hex)
                if len(payload) != 27:
                    raise ValueError("inject payload must be exactly 27 bytes")
            elif args.action == "lifecycle":
                payload = bytes((LIFECYCLE_REASONS[args.reason],))
            response = transport.exchange(COMMANDS[args.action], payload)
            result = describe_response(response)
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
