#!/usr/bin/env python3
"""Verify two local native-run receipts, without pretending to measure quality."""
import argparse
import array
import json
from pathlib import Path

from lyra_build import OUT, sha
from lyra_probe import INPUTS, MANIFEST_SHA, PACK, packets, read_pcm, record, save


def require(condition, message):
    if not condition:
        raise ValueError(message)


def verify(first, second):
    require(first.resolve() != second.resolve(), "two separate runs required")
    trace = json.loads((first / "packet-trace.json").read_text())
    require(set(trace) == {"version", "codec", "packet_ms", "lookahead_us", "flush_samples_8k",
                          "source_manifest_sha256", "library_sha256", "license", "attribution",
                          "samples"}, "trace fields")
    require((trace["version"], trace["codec"], trace["packet_ms"], trace["lookahead_us"],
             trace["flush_samples_8k"], trace["license"]) ==
            (1, "lyra-v2-3200", 20, None, 0, "CC-BY-SA-4.0"), "trace metadata")
    require(trace["source_manifest_sha256"] == sha(PACK / "manifest.json") == MANIFEST_SHA,
            "source manifest")
    require(trace["library_sha256"] == sha(first / "library-closure.json"), "closure hash")
    closure = json.loads((first / "library-closure.json").read_text())
    expected_paths = {"bin/encoder_main", "bin/decoder_main", "model_coeffs/lyra_config.binarypb",
                      "model_coeffs/lyragan.tflite", "model_coeffs/quantizer.tflite",
                      "model_coeffs/soundstream_encoder.tflite"}
    require({entry["path"] for entry in closure["files"]} == expected_paths, "closure inventory")
    for entry in closure["files"]:
        require(record(first / entry["path"], first) == entry, "binary/model mismatch")
        require(sha(first / entry["path"]) == sha(second / entry["path"]), "run closure mismatch")
    require([s["id"] for s in trace["samples"]] == list(INPUTS), "sample identities")
    stats = []
    for sample in trace["samples"]:
        sid = sample["id"]
        count, source_pcm = read_pcm(PACK / f"wav/{sid}-input8k.wav")
        require(sha(PACK / f"wav/{sid}-input8k.wav") == INPUTS[sid], "source WAV")
        require(sample["input_samples"] == count, "original sample count")
        raw = (first / f"{sid}-padded8k.lyra").read_bytes()
        require(sample["packets_hex"] == packets(raw, count), "actual packet bytes")
        padded_count, padded_pcm = read_pcm(first / f"{sid}-padded8k.wav")
        require(padded_pcm == source_pcm + bytes(((-count) % 160) * 2), "input preprocessing")
        decoded_count, pcm = read_pcm(first / f"{sid}-padded8k_decoded.wav")
        require(decoded_count == padded_count == len(sample["packets_hex"]) * 160, "sample cadence")
        require(any(pcm), "silent output")
        for suffix in ("padded8k.lyra", "padded8k_decoded.wav", "padded8k.wav", "original.wav", "input8k.wav"):
            require(sha(first / f"{sid}-{suffix}") == sha(second / f"{sid}-{suffix}"), "repeat differs")
        values = array.array("h", pcm)
        stats.append({"id": sid, "packets": len(sample["packets_hex"]),
                      "decoded_samples": decoded_count, "peak_absolute_pcm": max(abs(v) for v in values),
                      "nonzero_samples": sum(v != 0 for v in values)})
    for name in ("packet-trace.json", "library-closure.json", "index.html", "ATTRIBUTION.txt"):
        require(sha(first / name) == sha(second / name), "repeated artifact differs: " + name)
    return {"verified": True, "trace_sha256": sha(first / "packet-trace.json"),
            "library_closure_sha256": trace["library_sha256"], "samples": stats,
            "evidence": "two native CLI runs, identical encoded streams and decoded WAVs; not two clean builds",
            "human_quality": "not assessed", "hardware_executed": False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("first", type=Path)
    parser.add_argument("second", type=Path)
    args = parser.parse_args()
    result = verify(args.first, args.second)
    save(OUT / "verification.json", result)
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
