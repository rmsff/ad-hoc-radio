#!/usr/bin/env python3
"""Actual external Codec2 listening fixtures, independent of simulator runtime."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import sys

from bootstrap import BASE
from common import (MODES, PACKET_SAMPLES, digest, erase_packets, erasure_mask,
                    load_config, pcm_stats, read_pcm, restore_timeline, run, save, write_wav)

CONDITIONS = ("clean", "loss05", "loss20", "burst3")


def bound_path(root, ref):
    p = root / ref["path"]
    if not p.resolve().is_relative_to(root.resolve()) or p.is_symlink():
        raise ValueError("provenance path escapes root")
    if digest(p) != {k: ref[k] for k in ("sha256", "size")}:
        raise ValueError(f"provenance mismatch: {p}")
    return p


def prepare(config, tools, output):
    if sys.byteorder != "little":
        raise ValueError("upstream CLI uses native shorts; this pack requires a little-endian host")
    cfg = load_config(config)
    receipt_path = tools / "provenance.json"
    if receipt_path.stat().st_size > 2000000:
        raise ValueError("oversize provenance")
    receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    if receipt["codec2"] != cfg["codec2"] or receipt["config"] != digest(config):
        raise ValueError("source/config identity mismatch")
    binaries = {name: bound_path(tools, receipt["binaries"][name]) for name in ("c2enc", "c2dec")}
    for item in receipt["inputs"]:
        bound_path(tools / "inputs", item)
    if output.exists() or not output.resolve().is_relative_to(BASE.resolve()):
        raise ValueError("need a fresh output inside audio artifact root")
    # Validate all inputs before starting codec processes or creating any outputs.
    samples = []
    for sample in cfg["samples"]:
        sid = sample["id"]
        original = tools / "inputs" / (sid + ".ogg")
        if digest(original)["sha256"] != sample["sha256"]:
            raise ValueError("original source mismatch")
        pcm, _ = read_pcm(tools / "inputs" / f"{sid}-input8k.wav")
        reference, rate = read_pcm(tools / "inputs" / f"{sid}-original.wav", codec_input=False)
        if pcm_stats(pcm)["rms"] < 1:
            raise ValueError("silent input is not a speech fixture")
        samples.append((sample, pcm, reference, rate))
    output.mkdir(parents=True)
    (output / "wav").mkdir()
    (output / "evidence").mkdir()
    records, commands, blind = [], [], []
    for sample, pcm, reference, reference_rate in samples:
        sid = sample["id"]
        count = (len(pcm)//2 + PACKET_SAMPLES - 1) // PACKET_SAMPLES
        padded = pcm + bytes(count*PACKET_SAMPLES*2-len(pcm))
        rawpath = output / "evidence" / f"{sid}.raw"
        rawpath.write_bytes(padded)
        write_wav(output / "wav" / f"{sid}-input8k.wav", pcm)
        write_wav(output / "wav" / f"{sid}-original.wav", reference, reference_rate)
        variations = []
        for mode in MODES:
            bitpath = output / "evidence" / f"{sid}-{mode}.bit"
            commands.append(run([binaries["c2enc"], mode, rawpath, bitpath],
                                output / "evidence" / f"{sid}-{mode}-encode.log"))
            encoded = bitpath.read_bytes()
            if len(encoded) != count * mode * 80 // 8000:
                raise ValueError("c2enc did not produce exact mode/cadence bytes")
            for condition in CONDITIONS:
                mask = erasure_mask(count, condition, cfg["seed"], sid)
                received = erase_packets(encoded, mode, mask)
                stem = f"{sid}-{mode}-{condition}"
                recvpath = output / "evidence" / (stem + ".received.bit")
                recvpath.write_bytes(received)
                decpath = output / "evidence" / (stem + ".decoded.raw")
                if received:
                    commands.append(run([binaries["c2dec"], mode, recvpath, decpath],
                                        output / "evidence" / (stem + "-decode.log")))
                    decoded = decpath.read_bytes()
                else:
                    decoded = b""
                    decpath.write_bytes(decoded)
                reconstructed = restore_timeline(decoded, mask)[:len(pcm)]
                key = hashlib.sha256(f"blind-v1:{cfg['seed']}:{stem}".encode()).hexdigest()
                variations.append({"key": key, "mode": mode, "condition": condition, "pcm": reconstructed,
                    "packet_count": count, "packet_payload_bytes": mode*80//8000,
                    "codec_frame_samples": MODES[mode][0], "codec_frame_bytes": MODES[mode][1],
                    "erased_packet_indices": [i for i, erased in enumerate(mask) if erased],
                    "erased_packet_count": sum(mask), "erased_fraction": sum(mask)/count,
                    "received_bitstream": str(recvpath.relative_to(output)),
                    "decoder_pcm": str(decpath.relative_to(output))})
        tracks = []
        for i, var in enumerate(sorted(variations, key=lambda v: v["key"])):
            track = f"{sid}-{chr(65+i)}"
            outpcm = var.pop("pcm")
            var.pop("key")
            wavpath = output / "wav" / (track + ".wav")
            write_wav(wavpath, outpcm)
            row = {"id": track, "path": str(wavpath.relative_to(output)), **var, **pcm_stats(outpcm), **digest(wavpath)}
            records.append(row)
            tracks.append({"id": track, "path": row["path"], "mode": var["mode"], "condition": var["condition"]})
        blind.append({"id": sid, "speaker": sample["speaker"], "transcript": sample["transcript"],
                      "transcript_status": sample["transcript_status"], "tracks": tracks,
                      "original": f"wav/{sid}-original.wav", "reference": f"wav/{sid}-input8k.wav",
                      "duration_seconds": len(pcm)/16000})
    notices = []
    for s in cfg["samples"]:
        notices.append(f"{s['id']}: {s['source_page']}\nSpeaker: {s['speaker']}; text: {s['author']}.\n"
                       f"License: {s['license_url']} (original and audio derivatives).\n"
                       "Changes: Ogg decoded to PCM16 mono; 8 kHz resampling; Codec2; optional whole-packet erasure and silence insertion. No endorsement.\n")
    (output / "ATTRIBUTION.txt").write_text("\n".join(notices), encoding="utf-8")
    template = Path(__file__).with_name("listening.html").read_text(encoding="utf-8")
    data = json.dumps(blind, ensure_ascii=False).replace("<", "\\u003c").replace("&", "\\u0026")
    (output / "index.html").write_text(template.replace("__PACK_DATA__", data), encoding="utf-8")
    shutil.copy2(config, output / "evidence" / "source-config.json")
    shutil.copy2(receipt_path, output / "evidence" / "tool-provenance.json")
    manifest = {"schema_version": 1, "packet_ms": 80, "seed": cfg["seed"], "records": records,
        "boundary": "standalone external-process file conversion; not simulator playout integration",
        "source_kind": "two Russian human literary readings; one speaker; already Vorbis-compressed masters",
        "reference": "original-rate PCM decode and 8 kHz PCM codec input, not FM or channel measurements",
        "concealment": "whole 80 ms packets removed before c2dec; decoder state frozen across gaps; insert zero PCM at original packet times; no PLC, no crossfade",
        "padding": "zero pad end to 80 ms for c2enc, trim reconstructed WAV to original 8 kHz sample count",
        "loss_model": "SHA256 counter threshold pseudo-random packet erasures; shared mask across modes; burst3 is one seeded 240 ms outage",
        "hardware_executed": False, "runtime_integrated": False, "subjective_quality_verified": False,
        "human_scores_collected": False, "commands": commands,
        "generator_inputs": [{"path": str(p), **digest(p)} for p in sorted(Path(__file__).parent.glob("*.py"))] +
                            [{"path": str(Path(__file__).with_name("listening.html")), **digest(Path(__file__).with_name("listening.html"))}],
        "files": [{"path": str(p.relative_to(output)), **digest(p)} for p in sorted(output.rglob("*")) if p.is_file()]}
    save(output / "manifest.json", manifest)
    print(output / "index.html")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True, type=Path)
    parser.add_argument("--tools", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    prepare(args.config.resolve(), args.tools.resolve(), args.output.resolve())
