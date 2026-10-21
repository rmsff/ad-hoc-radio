#!/usr/bin/env python3
"""Collect source/model and exact firmware-target resource evidence, read-only."""
import json
from pathlib import Path
import subprocess

from lyra_build import OUT, ROOT, sha
from lyra_probe import INPUTS, MANIFEST_SHA, PACK, read_pcm, record, save


def main():
    upstream = OUT / "upstream"
    model = upstream / "lyra/model_coeffs"
    assets = [record(model / name, OUT) for name in
              ("lyra_config.binarypb", "lyragan.tflite", "quantizer.tflite",
               "soundstream_encoder.tflite")]
    target_root = ROOT / ".artifacts/mvp9.9-e-20260905-r2/mvp10.7-MDUV380_10W_PLUS_FW/build-a"
    firmware = target_root / "source/OpenGD77_MDUV380_DM1701_20260130/MDUV380_firmware"
    result_path = target_root / "output/build-result.json"
    target = json.loads(result_path.read_text())
    sections = target["resources"]["sections"]
    ram_end = sections["._user_heap_stack"]["address"] + sections["._user_heap_stack"]["size"]
    source = []
    if sha(PACK / "manifest.json") != MANIFEST_SHA:
        raise ValueError("source manifest changed")
    for sid, expected in INPUTS.items():
        path = PACK / f"wav/{sid}-input8k.wav"
        if sha(path) != expected:
            raise ValueError("input changed")
        count, _ = read_pcm(path)
        source.append({"id": sid, "input_samples": count, "input": record(path, ROOT),
                       "native_unpadded_frames": count // 160,
                       "native_unpadded_discarded_samples": count % 160,
                       "pad_to_frame_samples": (-count) % 160})
    licenses = [record(upstream / "LICENSE", OUT), record(OUT / "glog-0.6.0/COPYING", OUT)]
    evidence_files = [result_path, target_root / "output/size.txt",
                      target_root / "output/sections.txt",
                      target_root / "output/MDUV380_firmware.bin",
                      firmware / "STM32F405VGTX_FLASH.ld", firmware / "Core/Src/main.c",
                      firmware / "Core/Inc/stm32f4xx_hal_conf.h",
                      firmware / "Core/Inc/FreeRTOSConfig.h"]
    dependency_commits = {}
    for directory in sorted((OUT / "bazel-user").glob("*/external/*")):
        if (directory / ".git").exists():
            dependency_commits[directory.name] = subprocess.check_output(
                ["git", "rev-parse", "HEAD"], cwd=directory, text=True).strip()
    tracked = subprocess.check_output(["git", "ls-files", "-z"], cwd=upstream)
    source_closure = [record(upstream / name.decode(), upstream)
                      for name in tracked.split(b"\0") if name]
    save(OUT / "upstream-files.json", source_closure)
    inventory = {
        "upstream_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=upstream,
                                                   text=True).strip(),
        "upstream_dirty": subprocess.check_output(["git", "status", "--porcelain"], cwd=upstream,
                                                  text=True),
        "upstream_files": record(OUT / "upstream-files.json", OUT),
        "models": assets, "model_and_config_total_bytes": sum(a["size"] for a in assets),
        "licenses": licenses, "source_manifest_sha256": MANIFEST_SHA, "sources": source,
        "dependency_git_commits": dependency_commits,
        "resolved_gflags_commit_from_build_log": "57996515992ad933fd7f70bf2c9675cfe4d2e33f",
        "build_log": record(OUT / "build-native-3.log", OUT),
        "native_dynamic_libraries": subprocess.check_output(
            ["otool", "-L", str(OUT / "final-1/bin/encoder_main"),
             str(OUT / "final-1/bin/decoder_main")], text=True),
        "host": {"uname": subprocess.check_output(["uname", "-a"], text=True).strip(),
                 "cpu": subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip(),
                 "memory_bytes": int(subprocess.check_output(["sysctl", "-n", "hw.memsize"], text=True)),
                 "clang": subprocess.check_output(["clang", "--version"], text=True).strip()},
        "target": {"configuration": "MDUV380_10W_PLUS_FW", "mcu": target["mcu"],
                   "revision": "mvp9.9-e-20260905-r2", "resources": target["resources"],
                   "main_ram_region_bytes": 131072, "ccm_region_bytes": 65536,
                   "main_ram_unallocated_above_min_heap_stack_bytes": 0x20020000 - ram_end,
                   "ccm_unallocated_bytes": 65536 - sections[".ccmram"]["size"],
                   "flash_region_bytes": target["resources"]["flash_limit"] - target["resources"]["flash_origin"],
                   "configured_boot_clock_hz": 72000000,
                   "configured_boot_clock_derivation": "HSE_VALUE=8000000 / PLLM=4 * PLLN=72 / PLLP=2, AHB=1; not measured or maximum runtime clock",
                   "rtos_heap_reserved_bytes": 20480,
                   "runtime_free_ram_bytes": None, "lyra_mcu_cycles": None,
                   "artifacts": [record(p, ROOT) for p in evidence_files]},
        "measurement_boundary": "model storage and firmware static layout, not Lyra runtime RAM or MCU execution",
    }
    save(OUT / "inventory.json", inventory)
    print(json.dumps(inventory, indent=2))


if __name__ == "__main__":
    main()
