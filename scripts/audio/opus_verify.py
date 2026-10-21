"""Independently replay received Opus packets and verify aligned WAV bytes."""
import argparse
import json
from pathlib import Path

from common import digest, pcm_stats, read_pcm
from opus_codec import Opus, align, erase, loss_mask
from opus_generate import ROOT, SETTINGS, child, config, load_json, packet_stats


def require(condition, message):
    if not condition:
        raise ValueError(message)


def packets(values):
    require(isinstance(values, list) and 1 <= len(values) <= 3004, "packet count")
    require(all(v is None or isinstance(v, str) and 2 <= len(v) <= 2550 and len(v) % 2 == 0
                for v in values), "packet representation")
    return [bytes.fromhex(p) if p is not None else None for p in values]


def verify(root, check_sources=True):
    root = Path(root).resolve()
    manifest = load_json(child(root, "manifest.json"))
    require(manifest["version"] == 1 and manifest["settings"] == SETTINGS, "manifest settings")
    require(manifest["quality_status"] == "pending-human-listening" and not any(
        manifest[k] for k in ("hardware_executed", "codec_selected", "runtime_integrated")), "false acceptance")
    declared = set()
    for file in manifest["files"]:
        name = file["path"]
        require(name not in declared, "duplicate file")
        declared.add(name)
        path = child(root, name)
        require(path.stat().st_size <= 24_000_000, "oversize artifact")
        require(digest(path) == {k: file[k] for k in ("sha256", "size")}, f"hash mismatch: {name}")
    actual = {str(p.relative_to(root)) for p in root.rglob("*") if p.is_file()}
    require(actual == declared | {"manifest.json"}, "file inventory mismatch")
    cfg = config(child(root, "evidence/config.json"))
    require(cfg["library"] == manifest["library"] and cfg["seed"] == manifest["seed"], "config drift")
    require(digest(child(root, "evidence/source-manifest.json"))["sha256"] == cfg["source_manifest_sha256"]
            == manifest["source_manifest_sha256"], "source provenance")
    source = load_json(child(root, "evidence/source-manifest.json"))
    source_files = {f["path"]: {k: f[k] for k in ("sha256", "size")} for f in source["files"]}
    expected_refs = {f"wav/{sid}-{kind}.wav" for sid in cfg["samples"] for kind in ("original", "input8k")}
    require(set(manifest["source_references"]) == expected_refs, "missing reference")
    for name, expected in manifest["source_references"].items():
        require(digest(child(root, name)) == expected == source_files[name], "reference modified")
    if check_sources:
        for entry in manifest["generator_inputs"]:
            require(digest(child(ROOT, entry["path"])) == {k: entry[k] for k in ("sha256", "size")}, "tool source drift")
    opus = Opus(cfg["library"]["path"], cfg["library"]["sha256"])
    encoded = {}
    for encoding in manifest["encodings"]:
        sid, mode = encoding["sample_id"], encoding["encoder"]
        require(sid in cfg["samples"] and mode in ("fecoff", "fecon") and (sid, mode) not in encoded, "encoding identity")
        data = load_json(child(root, encoding["evidence"]))
        payload = packets(data["packets"])
        require(None not in payload, "missing original packet")
        pcm, _ = read_pcm(child(root, f"wav/{sid}-input8k.wav"))
        generated, meta = opus.encode(pcm, mode == "fecon")
        require(generated == payload and meta == data["meta"], "encode replay mismatch")
        require(packet_stats(meta) == encoding["stats"], "packet statistics mismatch")
        encoded[sid, mode] = (payload, meta)
    require(set(encoded) == {(s, e) for s in cfg["samples"] for e in ("fecoff", "fecon")}, "missing encoding")
    seen = set()
    for record in manifest["records"]:
        sid, mode, condition, fec = (record[k] for k in ("sample_id", "encoder", "condition", "use_fec"))
        key = sid, mode, condition, fec
        require(key not in seen, "duplicate record")
        seen.add(key)
        payload, meta = encoded[sid, mode]
        data = load_json(child(root, record["evidence"]))
        mask = loss_mask(len(payload), condition, cfg["seed"], sid)
        received = packets(data["received"])
        require(data["mask"] == mask and received == erase(payload, mask), "erasure mismatch")
        # Only the received list crosses the decoder boundary, never the source payload.
        decoded, actions = opus.decode_received(received, fec)
        require(actions == data["actions"], "decoder action mismatch")
        wav_pcm, _ = read_pcm(child(root, record["wav"]))
        require(wav_pcm == align(decoded, meta), "WAV decode mismatch")
        require(record["pcm"] == pcm_stats(wav_pcm) and record["pcm"]["rms"] > 0, "PCM stats/silence")
        expected = {"lost_packets": sum(mask), "actual_loss_percent": sum(mask)*100/len(mask),
                    "fec_recovered_packets": actions.count("fec"), "plc_packets": actions.count("plc"),
                    "fec_playout_wait_ms": 20 if fec else 0}
        require(all(record[k] == v for k, v in expected.items()), "recovery statistics")
    expected = {(s, e, "clean", False) for s in cfg["samples"] for e in ("fecoff", "fecon")}
    expected |= {(s, "fecon", c, f) for s in cfg["samples"] for c in ("loss05", "loss20", "loss30", "burst240") for f in (False, True)}
    require(seen == expected, "missing/extra listening conditions")
    page = load_json(child(root, "listening-data.json"))
    require(page["pack_id"] == "opus-listening-20260906-8k-nb-v1" and
            all(page["settings"].get(k) == v for k, v in SETTINGS.items()), "page settings")
    require([s["id"] for s in page["samples"]] == cfg["samples"], "page sample identity")
    for sample in page["samples"]:
        sid = sample["id"]
        meta = encoded[sid, "fecoff"][1]
        require(sample["duration_seconds"] == meta["input_samples"]/8000, "page duration")
        references = [t for t in sample["tracks"] if t["section"] == "reference"]
        require([t["path"] for t in references] == [f"wav/{sid}-{k}.wav" for k in ("original", "input8k")], "page original binding")
        require([t["label"] for t in references] == ["Оригинал", "Исходник 8 кГц"], "reference labels")
        candidates = [t for t in sample["tracks"] if t["section"] != "reference"]
        expected_records = [r for r in manifest["records"] if r["sample_id"] == sid and
                            (not r["use_fec"] or r["fec_recovered_packets"] > 0)]
        require([t["path"] for t in candidates] == [r["wav"] for r in expected_records], "page candidate binding")
        for t, r in zip(candidates, expected_records):
            require(t["condition"] == r["condition"] and
                    t["recovery"] == ("none" if r["lost_packets"] == 0 else
                                      "fec" if r["fec_recovered_packets"] else "plc"), "page recovery claim")
            require(t["section"] == ("clean" if r["encoder"] == "fecoff" else "loss"), "page section")
    if check_sources:
        template = Path(__file__).with_name("opus_listening.html").read_text(encoding="utf-8")
        inline = json.dumps(page, ensure_ascii=False, sort_keys=True).replace("&", "\\u0026").replace("<", "\\u003c")
        require((root / "index.html").read_text(encoding="utf-8") == template.replace("__PACK_DATA__", inline), "page differs from verified data/template")
    return {"verified": True, "records": len(seen), "wav_files": len(expected_refs)+len(seen),
            "human_quality": "not measured", "manifest": digest(root / "manifest.json")}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("pack", type=Path)
    args = parser.parse_args()
    print(json.dumps(verify(args.pack), indent=2))
