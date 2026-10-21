"""Pinned host codec adapters and explicit grouped erasure policies."""
import hashlib
import subprocess

from common import digest
from opus_codec import loss_mask
from opus_generate import ROOT, child

C2_HASHES = {"c2enc": "cf2a62ab7c32f41033d74b2030924d0cd55783c357a4c5a20f5956e8b93a9288",
             "c2dec": "03dd593132fa751c69aae98ce908ff0fffb1a3ac479570ce26b73979dd84beec"}


class Codec2:
    def __init__(self):
        self.tools = {}
        for name, sha in C2_HASHES.items():
            p = child(ROOT, f".artifacts/audio-listening-20260906/tools-r1/build/src/{name}")
            if digest(p)["sha256"] != sha:
                raise ValueError("Codec2 binary hash mismatch")
            self.tools[name] = p
        self.closure = hashlib.sha256("".join(C2_HASHES[k] for k in sorted(C2_HASHES)).encode()).hexdigest()

    def call(self, tool, mode, data):
        if mode not in (2400, 3200) or not data or len(data) > 960320:
            raise ValueError("Codec2 input outside bounds")
        p = subprocess.run([str(self.tools[tool]), str(mode), "-", "-"], input=data,
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=60, check=True)
        return p.stdout

    def encode(self, pcm, mode):
        if len(pcm) % 2 or not 16000 <= len(pcm) <= 960000:
            raise ValueError("Codec2 PCM duration/alignment")
        count = (len(pcm)//2+159)//160
        encoded = self.call("c2enc", mode, pcm + bytes(count*320-len(pcm)))
        size = mode//400
        if len(encoded) != count*size:
            raise ValueError("Codec2 packet count mismatch")
        return [encoded[i:i+size] for i in range(0, len(encoded), size)]

    def decode(self, received, mode):
        size = mode//400
        if not received or len(received)>3004 or any(p is not None and len(p)!=size for p in received):
            raise ValueError("Codec2 packet size/count")
        data = b"".join(p for p in received if p is not None)
        decoded = self.call("c2dec", mode, data) if data else b""
        if len(decoded) != sum(p is not None for p in received)*320:
            raise ValueError("Codec2 decoder duration")
        out, cursor = bytearray(), 0
        for p in received:
            if p is None:
                out.extend(bytes(320))
            else:
                out.extend(decoded[cursor:cursor+320]); cursor += 320
        return bytes(out)


def grouped_mask(count, group, condition, sid, seed=20260906):
    if type(count) is not int or not 1 <= count <= 3004 or group not in (1,2,4,8):
        raise ValueError("grouped mask bounds")
    groups = (count+group-1)//group
    if condition == "burst240":
        length = min(12//group, groups)
        if 12 % group:
            raise ValueError("240 ms burst must align to group duration")
        start = int.from_bytes(hashlib.sha256(f"codec-burst:{seed}:{sid}:{group}".encode()).digest()[:8], "big") % (groups-length+1)
        mask = [start <= i < start+length for i in range(groups)]
    else:
        mask = loss_mask(groups, condition, seed, sid)
    return [mask[i//group] for i in range(count)], mask
