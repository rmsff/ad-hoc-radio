"""Decrypt the segments of an official TYT OutSecurityBin firmware file.

Layout (as used for the 2026-07-09 MD-UV380 5W restore): region count at
offset 124, (start, length) pairs from offset 128, encrypted data from 0x100,
XOR with the 1024-byte MD-UV380 key indexed by the offset inside the data area.
The key is read from the OpenGD77 STM32 loader (MDUV380_ENCODE_CIPHER).
"""
import ast
import hashlib
import struct
import sys
from pathlib import Path

HEADER = b"OutSecurityBin"


def load_key(loader: Path) -> bytes:
    tree = ast.parse(loader.read_text())
    for node in tree.body:
        if isinstance(node, ast.AnnAssign) and getattr(node.target, "id", "") == "MDUV380_ENCODE_CIPHER":
            key = bytes(ast.literal_eval(node.value))
            if len(key) != 1024:
                raise SystemExit(f"unexpected key length {len(key)}")
            return key
    raise SystemExit("MDUV380_ENCODE_CIPHER not found")


def extract(firmware: Path, key: bytes, out_dir: Path | None) -> None:
    blob = firmware.read_bytes()
    if not blob.startswith(HEADER):
        raise SystemExit(f"{firmware.name}: not an OutSecurityBin file")
    count = struct.unpack("<I", blob[124:128])[0]
    regions = [struct.unpack("<II", blob[128 + 8 * i: 136 + 8 * i]) for i in range(count)]
    print(f"{firmware.name}: size={len(blob)} sha256={hashlib.sha256(blob).hexdigest()}")
    print("regions:", [(hex(start), hex(length)) for start, length in regions])
    base = 0
    for index, (start, length) in enumerate(regions):
        encrypted = blob[0x100 + base: 0x100 + base + length]
        if len(encrypted) != length:
            raise SystemExit(f"segment {index} truncated")
        plain = bytes(value ^ key[(base + i) % 1024] for i, value in enumerate(encrypted))
        stack, reset = struct.unpack("<II", plain[:8])
        print(f"  segment{index} start={start:#010x} len={length:#x} sha256={hashlib.sha256(plain).hexdigest()}"
              f" sp={stack:#010x} reset={reset:#010x}")
        if out_dir is not None:
            path = out_dir / f"segment{index}_{start:08x}_{length:08x}.bin"
            path.write_bytes(plain)
            print(f"    -> {path}")
        base += length
    trailer = len(blob) - 0x100 - base
    print(f"  bytes after segments: {trailer}")


def main() -> None:
    loader, firmware = Path(sys.argv[1]), Path(sys.argv[2])
    out_dir = Path(sys.argv[3]) if len(sys.argv) > 3 else None
    extract(firmware, load_key(loader), out_dir)


if __name__ == "__main__":
    main()
