#!/usr/bin/env python3
"""Verify delivered bytes, packet-erasure semantics, WAVs and optional replay."""
import argparse
import json
from pathlib import Path

from common import (PACKET_SAMPLES, digest, erase_packets, erasure_mask,
                    pcm_stats, read_pcm, restore_timeline)
from generate import bound_path


def verify(pack):
    value = json.loads((pack / "manifest.json").read_text(encoding="utf-8"))
    for ref in value["files"]:
        bound_path(pack, ref)
    for row in value["records"]:
        sid = row["id"].rsplit("-",1)[0]
        original, _ = read_pcm(pack / "wav" / f"{sid}-input8k.wav")
        pcm, _ = read_pcm(pack / row["path"])
        if pcm_stats(pcm) != {k:row[k] for k in pcm_stats(pcm)} or len(pcm) != len(original):
            raise ValueError("PCM duration/statistics mismatch")
        encoded = (pack / "evidence" / f"{sid}-{row['mode']}.bit").read_bytes()
        count = (len(original)//2+PACKET_SAMPLES-1)//PACKET_SAMPLES
        mask = erasure_mask(count,row["condition"],value["seed"],sid)
        if row["erased_packet_indices"] != [i for i,v in enumerate(mask) if v]:
            raise ValueError("erasure receipt mismatch")
        received = (pack / row["received_bitstream"]).read_bytes()
        if received != erase_packets(encoded,row["mode"],mask):
            raise ValueError("not actual packet erasure")
        decoded = (pack / row["decoder_pcm"]).read_bytes()
        if pcm != restore_timeline(decoded,mask)[:len(original)]:
            raise ValueError("silence concealment/timeline mismatch")
        if row["condition"] == "clean" and pcm_stats(pcm)["rms"] < 1:
            raise ValueError("silent codec output")
    return value


if __name__ == "__main__":
    p = argparse.ArgumentParser()
    p.add_argument("pack",type=Path)
    p.add_argument("--compare",type=Path)
    args = p.parse_args()
    receipt = verify(args.pack.resolve())
    if args.compare:
        verify(args.compare.resolve())
        for ref in receipt["files"]:
            if ref["path"].endswith((".wav",".bit",".raw")):
                if digest(args.pack/ref["path"]) != digest(args.compare/ref["path"]):
                    raise ValueError(f"audio/bitstream replay mismatch: {ref['path']}")
    print(f"PASS: {len(receipt['records'])} candidates, actual erasures, timeline and hashes; no quality verdict")
