"""Skip guard for tests that need gitignored local-only data."""
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
LISTENING_PACK = ROOT / ".artifacts/audio-listening-20260906/pack-a"


def require_local(path):
    """Skip a test when local-only data is absent, e.g. in a fresh clone."""
    if not Path(path).exists():
        raise unittest.SkipTest(f"local-only data not present: {Path(path).relative_to(ROOT)}")
