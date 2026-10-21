"""Compare an SPI flash snapshot with the 2026-07-14 baseline of the MD-UV380 10W."""
import hashlib
import sys
from pathlib import Path

BLOCK = 0x10000
REGIONS = {
    "codeplug 0x000000-0x040000": (0x000000, 0x040000),
    "D219.74 resources 0x060000-0x0bd400": (0x060000, 0x0BD400),
}


def main() -> int:
    baseline = Path(sys.argv[1]).read_bytes()
    snapshot = Path(sys.argv[2]).read_bytes()
    print(f"baseline sha256={hashlib.sha256(baseline).hexdigest()} size={len(baseline)}")
    print(f"snapshot sha256={hashlib.sha256(snapshot).hexdigest()} size={len(snapshot)}")
    if len(baseline) != len(snapshot):
        print("SIZE MISMATCH")
        return 1
    if baseline == snapshot:
        print("SPI: IDENTICAL to 2026-07-14 baseline")
        return 0
    for name, (start, end) in REGIONS.items():
        state = "identical" if baseline[start:end] == snapshot[start:end] else "DIFFERS"
        print(f"{name}: {state}")
    for start in range(0, len(baseline), BLOCK):
        old, new = baseline[start:start + BLOCK], snapshot[start:start + BLOCK]
        if old != new:
            changed = sum(a != b for a, b in zip(old, new))
            print(f"  block {start:#08x}: {changed} bytes differ")
    return 0


if __name__ == "__main__":
    sys.exit(main())
