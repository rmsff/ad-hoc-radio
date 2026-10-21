#!/usr/bin/env python3
"""Run actual pinned Lyra CLIs on the two verified source recordings."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess
import wave

from lyra_build import COMMIT, OUT, ROOT, environment, run, sha

PACK = ROOT / ".artifacts/audio-listening-20260906/pack-a"
MANIFEST_SHA = "5a1a9589df055e313bb3e19dbbf471f2b121f41a07a3c760b9be1a1c0c2555fc"
INPUTS = {
    "ru01": "c75b7e889e18a97c4f3a1b9dce4116ecd203e8a8c20b83d007d7b61723e88111",
    "ru02": "984c18a20b8bb0e97fd62cc6aafc0fb2725d56a412d5879507a6a07b525d35cb",
}


def packets(raw, input_samples):
    if type(input_samples) is not int or input_samples <= 0:
        raise ValueError("input_samples must be a positive integer")
    count = (input_samples + 159) // 160
    if not raw or len(raw) != count * 8:
        raise ValueError("expected exactly one 8-byte packet per padded 20 ms frame")
    return [raw[offset:offset + 8].hex() for offset in range(0, len(raw), 8)]


def read_pcm(path):
    with wave.open(str(path), "rb") as source:
        if (source.getnchannels(), source.getsampwidth(), source.getframerate(),
                source.getcomptype()) != (1, 2, 8000, "NONE"):
            raise ValueError("expected mono PCM16 8 kHz WAV")
        count = source.getnframes()
        pcm = source.readframes(count)
    if len(pcm) != count * 2 or count == 0:
        raise ValueError("truncated or empty WAV")
    return count, pcm


def padded_wav(source, target):
    count, pcm = read_pcm(source)
    padding = (-count) % 160
    with wave.open(str(target), "wb") as output:
        output.setparams((1, 2, 8000, 0, "NONE", "not compressed"))
        output.writeframes(pcm + bytes(padding * 2))
    return count, padding


def record(path, base):
    return {"path": str(path.relative_to(base)), "size": path.stat().st_size,
            "sha256": sha(path)}


def save(path, value):
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--label", default="probe-1")
    args = parser.parse_args()
    if not args.label.replace("-", "").isalnum():
        raise ValueError("label must be alphanumeric or hyphen")
    source = OUT / "upstream"
    if subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source,
                               text=True).strip() != COMMIT:
        raise ValueError("unexpected upstream commit")
    if sha(PACK / "manifest.json") != MANIFEST_SHA:
        raise ValueError("source manifest changed")
    manifest = json.loads((PACK / "manifest.json").read_text())
    inventory = {item["path"]: item for item in manifest["files"]}
    for sid, expected in INPUTS.items():
        name = f"wav/{sid}-input8k.wav"
        if sha(PACK / name) != expected or inventory[name]["sha256"] != expected:
            raise ValueError("source recording changed")
    native = source / "bazel-bin/lyra/cli_example"
    for name in ("encoder_main", "decoder_main"):
        if not (native / name).is_file():
            raise ValueError("native build missing; no trace can be generated")
    result = OUT / args.label
    result.mkdir(exist_ok=False)
    bins = result / "bin"
    bins.mkdir()
    for name in ("encoder_main", "decoder_main"):
        shutil.copy2(native / name, bins / name)
    models = result / "model_coeffs"
    models.mkdir()
    for name in ("lyra_config.binarypb", "quantizer.tflite", "lyragan.tflite",
                 "soundstream_encoder.tflite"):
        shutil.copy2(source / "lyra/model_coeffs" / name, models / name)
    shutil.copy2(source / "LICENSE", result / "LYRA-LICENSE.txt")
    shutil.copy2(PACK / "ATTRIBUTION.txt", result / "SOURCE-ATTRIBUTION.txt")
    (result / "ATTRIBUTION.txt").write_text(
        (PACK / "ATTRIBUTION.txt").read_text() +
        "\nLyra comparison derivatives: same PCM8k inputs; zero-padding to 20 ms; "
        "Lyra V2 1.3.2 3200 bit/s encode/decode with internal 8k/16k resampling. "
        "No new Codec2 processing, packet loss, normalization or delay trimming. "
        "Google LLC: Lyra code Apache-2.0; audio derivatives CC-BY-SA-4.0. "
        "No endorsement; human intelligibility has not been assessed.\n")
    shutil.copy2(PACK / "manifest.json", result / "source-manifest.json")
    closure = {"upstream_commit": COMMIT,
               "definition": "SHA256 of this manifest: actual native CLI binaries and four model/config assets; not an MCU library hash",
               "files": [record(p, result) for d in (bins, models)
                         for p in sorted(d.iterdir())]}
    save(result / "library-closure.json", closure)
    env = environment()
    details, samples = [], []
    for sid in INPUTS:
        for suffix in ("original", "input8k"):
            name = f"wav/{sid}-{suffix}.wav"
            if sha(PACK / name) != inventory[name]["sha256"]:
                raise ValueError("source reference changed")
            shutil.copy2(PACK / name, result / Path(name).name)
        wav = result / f"{sid}-padded8k.wav"
        count, padding = padded_wav(PACK / f"wav/{sid}-input8k.wav", wav)
        common = ["--bitrate=3200", "--model_path=" + str(models),
                  "--output_dir=" + str(result)]
        encoded = result / f"{sid}-padded8k.lyra"
        commands = [
            ("encode", [str(bins / "encoder_main"), "--input_path=" + str(wav),
                        "--enable_dtx=false", "--enable_preprocessing=false"] + common),
            ("decode", [str(bins / "decoder_main"), "--encoded_path=" + str(encoded),
                        "--sample_rate_hz=8000", "--packet_loss_rate=0",
                        "--randomize_num_samples_requested=false"] + common),
        ]
        for action, command in commands:
            label = f"{args.label}-{sid}-{action}"
            if run(["/usr/bin/time", "-l"] + command, label, source, 180, env):
                raise RuntimeError("native " + action + " failed; inspect " + label)
        payload = packets(encoded.read_bytes(), count)
        decoded = result / f"{sid}-padded8k_decoded.wav"
        decoded_count, _ = read_pcm(decoded)
        if decoded_count != len(payload) * 160:
            raise ValueError("decoder sample count mismatch")
        samples.append({"id": sid, "input_samples": count, "packets_hex": payload})
        details.append({"id": sid, "input_samples": count, "padding_samples": padding,
                        "decoded_samples": decoded_count, "packets": len(payload),
                        "input": record(wav, result), "encoded": record(encoded, result),
                        "decoded": record(decoded, result)})
    trace = {"version": 1, "codec": "lyra-v2-3200", "packet_ms": 20,
             "lookahead_us": None, "flush_samples_8k": 0,
             "source_manifest_sha256": MANIFEST_SHA,
             "library_sha256": sha(result / "library-closure.json"),
             "license": "CC-BY-SA-4.0",
             "attribution": (result / "ATTRIBUTION.txt").read_text(), "samples": samples}
    save(result / "packet-trace.json", trace)
    save(result / "probe-receipt.json", {"samples": details,
         "trace": record(result / "packet-trace.json", result),
         "lookahead_us": None, "alignment": "raw native decode; no delay trim or flush-tail claim",
         "quality_status": "pending-human-listening", "hardware_executed": False})
    shutil.copy2(Path(__file__).with_name("lyra_listening.html"), result / "index.html")
    print(json.dumps({"trace": str(result / "packet-trace.json"), "samples": details}))


if __name__ == "__main__":
    main()
