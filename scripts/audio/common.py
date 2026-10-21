"""Bounded offline audio helpers. No third-party Python or Go dependencies."""
import array
import hashlib
import json
import math
import os
from pathlib import Path
import re
import subprocess
import sys
import time
import wave

RATE = 8000
PACKET_SAMPLES = 640
MAX_SECONDS = 60
MODES = {1600: (320, 8), 3200: (160, 8)}


def digest(path):
    data = Path(path).read_bytes()
    return {"sha256": hashlib.sha256(data).hexdigest(), "size": len(data)}


def save(path, value):
    Path(path).write_text(json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2) + "\n", encoding="utf-8")


def load_config(path):
    if Path(path).stat().st_size > 65536:
        raise ValueError("config exceeds 64 KiB")
    value = json.loads(Path(path).read_text(encoding="utf-8"))
    if value["version"] != 1 or value["packet_ms"] != 80:
        raise ValueError("only v1/80 ms packetization is supported")
    if type(value["seed"]) is not int or not 0 <= value["seed"] < 2**64:
        raise ValueError("invalid seed")
    samples = value["samples"]
    if not 1 <= len(samples) <= 8:
        raise ValueError("need 1..8 samples")
    seen = set()
    for s in samples:
        if not re.fullmatch(r"[a-z][a-z0-9_-]{0,31}", s["id"]) or s["id"] in seen:
            raise ValueError("invalid/duplicate sample id")
        seen.add(s["id"])
        if not re.fullmatch(r"[0-9a-f]{64}", s["sha256"]):
            raise ValueError("invalid source hash")
        if s["kind"] != "human_recording" or s["license"] != "CC-BY-SA-4.0":
            raise ValueError("unsupported sample permission/type")
        if not 1 <= len(s["transcript"]) <= 8000:
            raise ValueError("invalid transcript")
    return value


def run(command, log, timeout=120):
    env = {**os.environ, "LC_ALL": "C", "LANG": "C", "TZ": "UTC"}
    start = time.monotonic()
    with Path(log).open("xb") as stream:
        result = subprocess.run([str(x) for x in command], stdout=stream,
                                stderr=subprocess.STDOUT, env=env, timeout=timeout, check=False)
    receipt = {"argv": [str(x) for x in command], "wall_seconds": round(time.monotonic()-start, 6),
               "exit_code": result.returncode, "log": Path(log).name}
    if result.returncode:
        raise ValueError(f"process failed ({result.returncode}); see {log}")
    return receipt


def read_pcm(path, codec_input=True):
    path = Path(path)
    if path.is_symlink() or path.stat().st_size > 24000000:
        raise ValueError("symlink/oversize WAV rejected")
    with wave.open(str(path), "rb") as w:
        if w.getcomptype() != "NONE" or w.getsampwidth() != 2 or w.getnchannels() != 1:
            raise ValueError("need uncompressed mono PCM16 WAV")
        rate, count = w.getframerate(), w.getnframes()
        if (codec_input and rate != RATE) or not 8000 <= rate <= 96000:
            raise ValueError("invalid sample rate")
        if not rate <= count <= MAX_SECONDS * rate:
            raise ValueError("duration outside 1..60 seconds")
        raw = w.readframes(count + 1)
        if len(raw) != count * 2:
            raise ValueError("truncated/inconsistent PCM data")
    return raw, rate


def write_wav(path, pcm, rate=RATE):
    if len(pcm) % 2:
        raise ValueError("unaligned PCM")
    with wave.open(str(path), "wb") as w:
        w.setparams((1, 2, rate, 0, "NONE", "not compressed"))
        w.writeframes(pcm)


def pcm_stats(raw, rate=RATE):
    values = array.array("h", raw)
    if sys.byteorder != "little":
        values.byteswap()
    rms = math.sqrt(sum(v*v for v in values)/len(values)) if values else 0
    return {"samples": len(values), "rate_hz": rate, "duration_seconds": len(values)/rate,
            "peak_abs": max((abs(v) for v in values), default=0), "rms": round(rms, 6),
            "clipped_samples": sum(v in (-32768, 32767) for v in values)}


def erasure_mask(count, condition, seed, sample_id):
    if type(count) is not int or not 1 <= count <= 750:
        raise ValueError("packet count outside 1..750")
    if type(seed) is not int or not 0 <= seed < 2**64:
        raise ValueError("invalid seed")
    if condition not in ("clean", "loss05", "loss20", "burst3"):
        raise ValueError("unknown condition")
    prefix = f"audio-erasure-v1:{seed}:{sample_id}:".encode("ascii")
    if condition == "burst3":
        length = min(3, count)
        start = int.from_bytes(hashlib.sha256(prefix+b"burst").digest()[:8], "big") % (count-length+1)
        return [start <= i < start+length for i in range(count)]
    pct = {"clean": 0, "loss05": 5, "loss20": 20}[condition]
    threshold = pct * 2**64 // 100
    return [int.from_bytes(hashlib.sha256(prefix+str(i).encode("ascii")).digest()[:8], "big") < threshold
            for i in range(count)]


def erase_packets(encoded, mode, mask):
    if mode not in MODES or any(type(v) is not bool for v in mask):
        raise ValueError("invalid mode/mask")
    size = mode * 80 // 8000
    if not mask or len(encoded) != len(mask) * size:
        raise ValueError("encoded length does not match packet count")
    return b"".join(encoded[i*size:(i+1)*size] for i, lost in enumerate(mask) if not lost)


def restore_timeline(decoded, mask):
    packet_bytes = PACKET_SAMPLES * 2
    if len(decoded) != mask.count(False)*packet_bytes:
        raise ValueError("decoder output length mismatch")
    out, cursor = bytearray(), 0
    for lost in mask:
        if lost:
            out.extend(bytes(packet_bytes))
        else:
            out.extend(decoded[cursor:cursor+packet_bytes])
            cursor += packet_bytes
    return bytes(out)
