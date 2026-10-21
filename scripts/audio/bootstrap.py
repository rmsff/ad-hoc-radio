#!/usr/bin/env python3
"""Explicit optional download/build; outputs stay inside the audio artifact root."""
import argparse
from pathlib import Path
import platform
import shutil
import tarfile
import urllib.parse
import urllib.request

from common import digest, load_config, read_pcm, run, save

BASE = Path(__file__).resolve().parents[2] / ".artifacts/audio-listening-20260906"


def fetch(url, dest, sha, limit):
    if urllib.parse.urlparse(url).hostname not in ("codeload.github.com", "upload.wikimedia.org"):
        raise ValueError("download host not allowed")
    if dest.exists():
        if dest.is_symlink() or dest.stat().st_size > limit or digest(dest)["sha256"] != sha:
            raise ValueError(f"cached input mismatch: {dest}")
        return
    req = urllib.request.Request(url, headers={"User-Agent": "ADHR-local-audio-evaluation/1.0"})
    with urllib.request.urlopen(req, timeout=60) as response:
        data = response.read(limit+1)
    if len(data) > limit:
        raise ValueError("download exceeds bound")
    import hashlib
    if hashlib.sha256(data).hexdigest() != sha:
        raise ValueError("download hash mismatch")
    with dest.open("xb") as stream:
        stream.write(data)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    cfg = load_config(args.config)
    output = args.output.resolve()
    if not output.is_relative_to(BASE.resolve()) or output == BASE.resolve():
        raise ValueError("output must be a new directory under the audio artifact root")
    output.mkdir(parents=True, exist_ok=False)
    cache = BASE / "intake"
    cache.mkdir(exist_ok=True)
    codec = cfg["codec2"]
    archive = cache / "codec2-1.2.0.tar.gz"
    fetch(codec["url"], archive, codec["sha256"], 50000000)
    source = output / "source"
    source.mkdir()
    with tarfile.open(archive) as tar:
        members = tar.getmembers()
        if len(members) > 10000 or sum(m.size for m in members) > 100000000:
            raise ValueError("source archive exceeds bound")
        for m in members:
            p = Path(m.name)
            if p.is_absolute() or ".." in p.parts or not (m.isfile() or m.isdir()):
                raise ValueError("unsafe source archive member")
        tar.extractall(source, filter="data")
    project = source / ("codec2-" + codec["commit"])
    build = output / "build"
    cmds = []
    cmds.append(run(["cmake", "-S", project, "-B", build, "-DCMAKE_BUILD_TYPE=Release",
                     "-DBUILD_SHARED_LIBS=OFF", "-DLPCNET=OFF", "-DUNITTEST=OFF"], output / "configure.log"))
    cmds.append(run(["cmake", "--build", build, "--target", "c2enc", "c2dec", "--parallel", "4"],
                    output / "build.log", timeout=300))
    inputs = output / "inputs"
    inputs.mkdir()
    for s in cfg["samples"]:
        original = cache / (s["id"] + ".ogg")
        fetch(s["url"], original, s["sha256"], 2000000)
        shutil.copy2(original, inputs / original.name)
        for suffix, extra in (("original", []), ("input8k", ["-ar", "8000"])):
            dest = inputs / f"{s['id']}-{suffix}.wav"
            cmds.append(run(["ffmpeg", "-nostdin", "-v", "error", "-i", original, "-map_metadata", "-1",
                             "-ac", "1", *extra, "-t", "61", "-c:a", "pcm_s16le", "-fflags", "+bitexact",
                             "-flags:a", "+bitexact", dest], output / f"{s['id']}-{suffix}.log"))
            read_pcm(dest, codec_input=suffix == "input8k")
    tool_versions = {}
    for name, flag in (("cmake", "--version"), ("cc", "--version"), ("ffmpeg", "-version")):
        cmds.append(run([name, flag], output / f"{name}-version.log"))
        tool_versions[name] = {"path": shutil.which(name), "version_log": f"{name}-version.log",
                               **digest(Path(shutil.which(name)).resolve())}
    binaries = {name: {"path": str((build / "src" / name).relative_to(output)),
                        **digest(build / "src" / name)} for name in ("c2enc", "c2dec")}
    save(output / "provenance.json", {"schema_version": 1, "codec2": codec, "config": digest(args.config),
         "platform": platform.platform(), "python": platform.python_version(), "source_archive": digest(archive),
         "license": digest(project / "COPYING"), "binaries": binaries, "tools": tool_versions,
         "commands": cmds, "inputs": [{"path": p.name, **digest(p)} for p in sorted(inputs.iterdir())],
         "source_files": [{"path": str(p.relative_to(project)), **digest(p)}
                          for p in sorted(project.rglob("*")) if p.is_file()],
         "hardware_executed": False, "runtime_integrated": False})
    print(output / "provenance.json")


if __name__ == "__main__":
    main()
