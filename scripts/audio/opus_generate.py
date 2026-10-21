"""Prepare an isolated, reproducible Opus listening pack; never touches radios."""
import argparse
import json
import math
from pathlib import Path
import re
import shutil

from common import digest, pcm_stats, read_pcm, save, write_wav
from opus_codec import Opus, align, erase, loss_mask

ROOT = Path(__file__).resolve().parents[2]
SETTINGS = {"target_bitrate_bps": 8000, "rate_hz": 8000, "packet_ms": 20,
            "channels": 1, "application": "voip", "bandwidth": "narrowband",
            "vbr": "constrained", "complexity": 10, "dtx": False,
            "fec_encoder_loss_percent": 30}
CONDITIONS = ("loss05", "loss20", "loss30", "burst240")
LABELS = {"loss05": "Потери 5%", "loss20": "Потери 20%", "loss30": "Потери 30%",
          "burst240": "Пропуск 240 мс"}


def load_json(path, limit=2_000_000):
    path = Path(path)
    if path.is_symlink() or path.stat().st_size > limit:
        raise ValueError("oversize/symlink JSON")
    return json.loads(path.read_text(encoding="utf-8"))


def child(root, name):
    relative = Path(name)
    if relative.is_absolute() or not relative.parts or any(p in ("..", ".") for p in relative.parts):
        raise ValueError("unsafe artifact path")
    result = root / relative
    if any(p.is_symlink() for p in (result, *result.parents)) or not result.resolve().is_relative_to(root.resolve()):
        raise ValueError("symlink/path escape")
    return result


def config(path):
    value = load_json(path, 65536)
    if value["version"] != 1 or value["settings"] != SETTINGS:
        raise ValueError("unsupported settings; never silently ignore config")
    ids = value["samples"]
    if (not isinstance(ids, list) or not 1 <= len(ids) <= 8 or
            any(not isinstance(i, str) or not re.fullmatch(r"[a-z][a-z0-9_-]{0,31}", i) for i in ids) or
            len(ids) != len(set(ids))):
        raise ValueError("invalid/duplicate sample IDs")
    loss_mask(1, "clean", value["seed"], ids[0])
    return value


def source_inputs(cfg):
    root = child(ROOT, cfg["source_pack"])
    manifest_path = child(root, "manifest.json")
    if digest(manifest_path)["sha256"] != cfg["source_manifest_sha256"]:
        raise ValueError("source manifest changed")
    manifest = load_json(manifest_path)
    files = {f["path"]: {k: f[k] for k in ("sha256", "size")} for f in manifest["files"]}
    names = [f"wav/{i}-{kind}.wav" for i in cfg["samples"] for kind in ("original", "input8k")]
    names += ["ATTRIBUTION.txt", "evidence/source-config.json"]
    for name in names:
        item = child(root, name)
        if item.stat().st_size > 24_000_000 or digest(item) != files[name]:
            raise ValueError(f"source mismatch: {name}")
        if name.endswith(".wav"):
            read_pcm(item, codec_input=name.endswith("input8k.wav"))
    return root, files


def packet_stats(meta):
    sizes = meta["packet_lengths"]
    count = len(sizes)
    return {"packets": count, "lbrr_packets": sum(meta["lbrr"]),
            "payload_bytes": sum(sizes), "mean_payload_bps": sum(sizes)*8/(count*.020),
            "min_bytes": min(sizes), "p95_bytes": sorted(sizes)[math.ceil(.95*count)-1],
            "max_bytes": max(sizes), "lookahead_ms": meta["lookahead_samples"]/8,
            "tail_padding_samples": count*160-meta["input_samples"]-meta["lookahead_samples"]}


def render_case(opus, output, sample, packets, meta, condition, fec, encoder):
    sid = sample["id"]
    mask = loss_mask(len(packets), condition, sample["seed"], sid)
    received = erase(packets, mask)
    decoded, actions = opus.decode_received(received, fec)
    stem = f"{sid}-{encoder}-{condition}-{'fec' if fec else 'plc'}"
    evidence, wav = f"evidence/{stem}.json", f"wav/{stem}.wav"
    save(output / evidence, {"received": [p.hex() if p is not None else None for p in received],
                             "mask": mask, "actions": actions})
    pcm = align(decoded, meta)
    write_wav(output / wav, pcm)
    return {"sample_id": sid, "encoder": encoder, "condition": condition,
            "use_fec": fec, "evidence": evidence, "wav": wav, "pcm": pcm_stats(pcm),
            "lost_packets": sum(mask), "actual_loss_percent": sum(mask)*100/len(mask),
            "fec_recovered_packets": actions.count("fec"), "plc_packets": actions.count("plc"),
            "fec_playout_wait_ms": 20 if fec else 0}


def track(record, label, section):
    return {"id": Path(record["wav"]).stem, "label": label, "path": record["wav"],
            "section": section, "condition": record["condition"],
            "recovery": ("none" if record["lost_packets"] == 0 else
                         "fec" if record["fec_recovered_packets"] else "plc")}


def generate(config_path, output):
    cfg = config(config_path)
    source, files = source_inputs(cfg)
    opus = Opus(cfg["library"]["path"], cfg["library"]["sha256"])
    if opus.version != cfg["library"]["version"]:
        raise ValueError("library version mismatch")
    template_path = Path(__file__).with_name("opus_listening.html")
    template = template_path.read_text(encoding="utf-8")
    if template.count("__PACK_DATA__") != 1:
        raise ValueError("invalid HTML template")
    license_text = Path(cfg["library"]["license_path"]).read_text(encoding="utf-8")
    output = Path(output).absolute()
    artifact_root = ROOT / ".artifacts/opus-listening-20260906"
    if not output.is_relative_to(artifact_root) or output == artifact_root:
        raise ValueError("output must be a fresh directory under the Opus artifact root")
    child(ROOT, str(output.relative_to(ROOT)))
    output.mkdir(parents=True, exist_ok=False)
    (output / "wav").mkdir()
    (output / "evidence").mkdir()
    shutil.copyfile(config_path, output / "evidence/config.json")
    shutil.copyfile(source / "manifest.json", output / "evidence/source-manifest.json")
    shutil.copyfile(source / "evidence/source-config.json", output / "evidence/source-config.json")
    (output / "OPUS-COPYING.txt").write_text(license_text, encoding="utf-8")
    original_attribution = (source / "ATTRIBUTION.txt").read_text(encoding="utf-8")
    attribution = original_attribution.replace(
        "Codec2; optional whole-packet erasure and silence insertion.",
        "Opus 8 kbit/s NB; optional 20 ms packet erasures and native PLC/FEC decoding.")
    (output / "ATTRIBUTION.txt").write_text(attribution, encoding="utf-8")
    records, encodings, samples = [], [], []
    for sid in cfg["samples"]:
        pcm, _ = read_pcm(source / f"wav/{sid}-input8k.wav")
        sample = {"id": sid, "seed": cfg["seed"], "duration_seconds": len(pcm)/16000, "tracks": []}
        for kind, label in (("original", "Оригинал"), ("input8k", "Исходник 8 кГц")):
            name = f"wav/{sid}-{kind}.wav"
            shutil.copyfile(source / name, output / name)
            sample["tracks"].append({"id": f"{sid}-{kind}", "label": label, "path": name,
                                     "section": "reference", "condition": "reference", "recovery": "none"})
        for enabled in (False, True):
            encoder = "fecon" if enabled else "fecoff"
            packets, meta = opus.encode(pcm, enabled)
            evidence = f"evidence/{sid}-{encoder}-encoded.json"
            save(output / evidence, {"packets": [p.hex() for p in packets], "meta": meta})
            encodings.append({"sample_id": sid, "encoder": encoder, "evidence": evidence,
                              "stats": packet_stats(meta)})
            clean = render_case(opus, output, sample, packets, meta, "clean", False, encoder)
            records.append(clean)
            label = "Opus 8 кбит/с · без потерь" if not enabled else "Без потерь · FEC запрошен"
            sample["tracks"].append(track(clean, label, "clean" if not enabled else "loss"))
            if enabled:
                for condition in CONDITIONS:
                    pair = [render_case(opus, output, sample, packets, meta, condition, fec, encoder)
                            for fec in (False, True)]
                    records.extend(pair)
                    actual = pair[0]["actual_loss_percent"]
                    label = f"{LABELS[condition]} · фактически {actual:.1f}% · PLC"
                    sample["tracks"].append(track(pair[0], label, "loss"))
                    # Keep the fallback WAV in evidence, without duplicate identical players.
                    if pair[1]["fec_recovered_packets"]:
                        sample["tracks"].append(track(pair[1], f"{LABELS[condition]} · FEC + PLC", "loss"))
        del sample["seed"]
        samples.append(sample)
    total_lbrr = sum(e["stats"]["lbrr_packets"] for e in encodings)
    data = {"pack_id": "opus-listening-20260906-8k-nb-v1", "samples": samples, "settings": {
        **SETTINGS, "fec_note": ("В этих записях при 8 кбит/с кодек не добавил FEC-данные. "
        "Варианты с потерями используют PLC, а не FEC." if total_lbrr == 0 else
        "FEC применяется только при наличии данных в следующем полученном пакете.")}}
    save(output / "listening-data.json", data)
    encoded = json.dumps(data, ensure_ascii=False, sort_keys=True).replace("&", "\\u0026").replace("<", "\\u003c")
    (output / "index.html").write_text(template.replace("__PACK_DATA__", encoded), encoding="utf-8")
    inputs = [Path(__file__), Path(__file__).with_name("opus_codec.py"),
              Path(__file__).with_name("opus_verify.py"), Path(__file__).with_name("common.py"), template_path]
    manifest = {"version": 1, "settings": SETTINGS, "seed": cfg["seed"], "library": cfg["library"],
                "source_manifest_sha256": cfg["source_manifest_sha256"], "encodings": encodings,
                "records": records, "source_references": {k: v for k, v in files.items() if
                    k in [t["path"] for s in samples for t in s["tracks"] if t["section"] == "reference"]},
                "generator_inputs": [{"path": str(p.relative_to(ROOT)), **digest(p)} for p in inputs],
                "quality_status": "pending-human-listening", "hardware_executed": False,
                "codec_selected": False, "runtime_integrated": False,
                "loss_model": "seeded independent whole 20 ms erasures; shared mask across PLC/FEC; includes flush tail",
                "alignment": "trim declared encoder lookahead and flush tail; no normalization/denoising; not real-time latency"}
    manifest["files"] = [{"path": str(p.relative_to(output)), **digest(p)} for p in sorted(output.rglob("*")) if p.is_file()]
    save(output / "manifest.json", manifest)
    return {"output": str(output), "encodings": encodings, "wav_files": len(list((output / "wav").glob("*.wav")))}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    print(json.dumps(generate(args.config, args.output), indent=2))
