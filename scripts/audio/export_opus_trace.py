"""Export verified FEC-off packets, not nominal lengths or a codec acceptance."""
import argparse
import json
from pathlib import Path
import re

from common import digest
from opus_codec import FRAME, MAX_PACKET_BYTES, MAX_PACKETS, RATE
from opus_generate import SETTINGS, child
import opus_verify

MAX_JSON_BYTES = 2_000_000
LOOKAHEAD_SAMPLES = 52
LICENSE = "CC-BY-SA-4.0"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, f"duplicate JSON key: {key}")
        result[key] = value
    return result


def reject_constant(value):
    raise ValueError(f"invalid JSON constant: {value}")


def read_json(path):
    with path.open("rb") as stream:
        raw = stream.read(MAX_JSON_BYTES + 1)
    require(len(raw) <= MAX_JSON_BYTES, "oversize JSON")
    value = json.loads(raw, object_pairs_hook=unique_object, parse_constant=reject_constant)
    require(isinstance(value, dict), "JSON object required")
    return value


def sample_ids(values):
    require(isinstance(values, list) and 1 <= len(values) <= 8, "sample count")
    require(all(isinstance(sid, str) and re.fullmatch(r"[a-z][a-z0-9_-]{0,31}", sid)
                for sid in values), "unsafe sample ID")
    require(len(values) == len(set(values)), "duplicate sample ID")
    return values


def sha256(value):
    require(isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value), "invalid SHA-256")
    return value


def encoding_sample(sid, entry, checked_file):
    data = read_json(checked_file(entry["evidence"]))
    meta, values = data["meta"], data["packets"]
    count = meta["input_samples"]
    require(type(count) is int and RATE <= count <= 60 * RATE, "input duration must be 1..60 seconds")
    require(type(meta["lookahead_samples"]) is int and
            meta["lookahead_samples"] == LOOKAHEAD_SAMPLES, "unexpected lookahead")
    require(isinstance(values, list) and 1 <= len(values) <= MAX_PACKETS, "packet count")
    require(len(values) == (count + LOOKAHEAD_SAMPLES + FRAME - 1) // FRAME,
            "packet count does not include exact lookahead/flush tail")
    lengths = []
    for value in values:
        require(isinstance(value, str) and 2 <= len(value) <= 2 * MAX_PACKET_BYTES and
                len(value) % 2 == 0 and re.fullmatch(r"[0-9a-fA-F]+", value), "invalid packet hex/length")
        lengths.append(len(bytes.fromhex(value)))
    require(meta["packet_lengths"] == lengths, "packet lengths mismatch")
    require(entry["stats"]["packets"] == len(values), "declared packet count mismatch")
    return {"id": sid, "input_samples": count, "packets_hex": values}


def collect_trace(root):
    """Bound and bind export inputs; native verification is still mandatory."""
    manifest_path = child(root, "manifest.json")
    manifest = read_json(manifest_path)
    require(type(manifest["version"]) is int and manifest["version"] == 1 and
            manifest["settings"] == SETTINGS, "unsupported manifest settings")
    files = {}
    for entry in manifest["files"]:
        name = entry["path"]
        require(name not in files, "duplicate artifact")
        files[name] = {key: entry[key] for key in ("sha256", "size")}

    def checked_file(name):
        item = child(root, name)
        require(name in files and item.stat().st_size <= MAX_JSON_BYTES and
                digest(item) == files[name], f"source artifact mismatch: {name}")
        return item

    cfg = read_json(checked_file("evidence/config.json"))
    ids = sample_ids(cfg["samples"])
    require(cfg["settings"] == SETTINGS and cfg["library"] == manifest["library"], "config drift")
    library_sha = sha256(cfg["library"]["sha256"])
    source_path = checked_file("evidence/source-manifest.json")
    require(digest(source_path)["sha256"] == sha256(manifest["source_manifest_sha256"])
            == cfg["source_manifest_sha256"], "source manifest changed")
    source = read_json(source_path)
    source_config_path = checked_file("evidence/source-config.json")
    source_entries = [f for f in source["files"] if f["path"] == "evidence/source-config.json"]
    require(len(source_entries) == 1 and digest(source_config_path) ==
            {key: source_entries[0][key] for key in ("sha256", "size")}, "source attribution provenance")
    originals = read_json(source_config_path)["samples"]
    sample_ids([item["id"] for item in originals])
    originals = {item["id"]: item for item in originals}
    attribution = checked_file("ATTRIBUTION.txt").read_text(encoding="utf-8")
    require(attribution.strip() and "Opus" in attribution and "No endorsement" in attribution,
            "missing conversion/endorsement attribution")
    references = []
    for sid in ids:
        require(sid in originals, "missing source attribution")
        original = originals[sid]
        require(original["license"] == LICENSE, "unsupported source license")
        for key in ("source_page", "speaker", "author", "license_url"):
            require(isinstance(original[key], str) and original[key] and original[key] in attribution,
                    f"missing attribution: {key}")
        for key in ("url", "text_url"):
            require(isinstance(original[key], str) and original[key].startswith("https://"),
                    f"missing source URL: {key}")
        references.append(f"{sid} original audio: {original['url']}\nText source: {original['text_url']}")

    encodings = {}
    for entry in manifest["encodings"]:
        sid, mode = entry["sample_id"], entry["encoder"]
        require(sid in ids and mode in ("fecoff", "fecon"), "encoding identity")
        require((sid, mode) not in encodings, "duplicate encoding")
        encodings[sid, mode] = entry
    require(all((sid, "fecoff") in encodings for sid in ids), "missing FEC-off stream")
    samples = [encoding_sample(sid, encodings[sid, "fecoff"], checked_file) for sid in ids]
    return {
        "version": 1,
        "codec": "opus-8000-nb-20ms-cvbr",
        # This is the Opus receipt, which itself binds the earlier PCM source receipt.
        "source_manifest_sha256": digest(manifest_path)["sha256"],
        "library_sha256": library_sha,
        "packet_ms": 20,
        "lookahead_us": 6500,
        "license": LICENSE,
        "attribution": attribution + "\n" + "\n\n".join(references) +
                       "\nTrace conversion: ordered FEC-off Opus packets exported as hexadecimal JSON. "
                       "CC-BY-SA-4.0 applies to the audio derivative. No endorsement.\n",
        "samples": samples,
    }


def export(pack, output):
    root, output = Path(pack).resolve(), Path(output).absolute()
    if output.exists() or output.is_symlink():
        raise FileExistsError(f"output must be fresh: {output}")
    require(not output.resolve().is_relative_to(root), "output must not alter the source pack")
    trace = collect_trace(root)
    receipt = opus_verify.verify(root)
    require(receipt["verified"] is True and
            receipt["manifest"]["sha256"] == trace["source_manifest_sha256"], "verification receipt mismatch")
    require(collect_trace(root) == trace, "source changed during verification")
    serialized = json.dumps(trace, ensure_ascii=True, indent=2) + "\n"
    with output.open("x", encoding="utf-8") as stream:
        stream.write(serialized)
    return trace


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pack", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    try:
        trace = export(args.pack, args.output)
    except (OSError, ValueError, KeyError, TypeError) as error:
        parser.exit(1, f"Opus trace export rejected: {error}\n")
    print(json.dumps({"output": str(args.output), "samples": len(trace["samples"]),
                      "packets": sum(len(s["packets_hex"]) for s in trace["samples"]),
                      "source_manifest_sha256": trace["source_manifest_sha256"]}, indent=2))


if __name__ == "__main__":
    main()
