#!/usr/bin/env python3
"""Isolated, bounded native Lyra V2 host build. No global package installation."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import time

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / ".artifacts/codec-comparison-20260906/lyra"
COMMIT = "47698dadf0010abff6a848e02642f55f806d4842"


def sha(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def environment():
    env = dict(os.environ)
    for name, leaf in (("HOME", "home"), ("TMPDIR", "tmp"),
                       ("XDG_CACHE_HOME", "cache")):
        directory = OUT / leaf
        directory.mkdir(parents=True, exist_ok=True)
        env[name] = str(directory)
    env.update(PYTHONDONTWRITEBYTECODE="1", LC_ALL="C", LANG="C")
    return env


def run(command, name, cwd, timeout, env):
    started = time.time()
    log = OUT / (name + ".log")
    receipt = {"command": command, "cwd": str(cwd), "timeout_s": timeout}
    print(json.dumps(receipt), flush=True)
    with log.open("wb") as stream:
        process = subprocess.Popen(command, cwd=cwd, env=env, stdout=stream,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        try:
            receipt["returncode"] = process.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            import signal
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
            receipt.update(returncode=124, timed_out=True)
    receipt["elapsed_s"] = time.time() - started
    receipt["log_sha256"] = sha(log)
    (OUT / (name + ".json")).write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps(receipt), flush=True)
    return receipt["returncode"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--timeout", type=int, default=600)
    parser.add_argument("--label", default="build-native-1")
    parser.add_argument("--python", type=Path)
    parser.add_argument("--glog-060", action="store_true")
    args = parser.parse_args()
    if not args.label.replace("-", "").isalnum():
        raise ValueError("label must be alphanumeric or hyphen")
    if (OUT / (args.label + ".json")).exists():
        raise ValueError("use a fresh build receipt label")
    env = environment()
    if args.python:
        env["PYTHON_BIN_PATH"] = str(args.python.resolve())
        env["PATH"] = str(args.python.parent) + os.pathsep + env["PATH"]
    source = OUT / "upstream"
    actual = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source,
                                     text=True).strip()
    if actual != COMMIT:
        raise ValueError("unexpected upstream commit")
    if platform.system() != "Darwin" or platform.machine() != "arm64":
        raise ValueError("this probe pins the macOS arm64 Bazel distribution")
    binary = OUT / "bazel-5.3.2-darwin-arm64"
    if not binary.exists():
        url = "https://github.com/bazelbuild/bazel/releases/download/5.3.2/" + binary.name
        if run(["curl", "-fL", "--retry", "2", "--max-time", "120", url,
                "-o", str(binary)], "download-bazel", OUT, 150, env):
            raise RuntimeError("Bazel download failed")
        binary.chmod(0o755)
    if sha(binary) != "d3ce116379b835e2a0ae16b924a485c48330db434b192f090c8f875238e584cb":
        raise ValueError("Bazel binary differs from the recorded upstream release download")
    platform_receipt = {"platform": platform.platform(), "machine": platform.machine(),
                        "lyra_commit": COMMIT, "bazel_sha256": sha(binary),
                        "python_bin_path": env.get("PYTHON_BIN_PATH")}
    (OUT / "platform.json").write_text(json.dumps(platform_receipt, indent=2) + "\n")
    command = [str(binary), "--batch", "--output_user_root=" + str(OUT / "bazel-user"),
               "--host_jvm_args=-Djava.io.tmpdir=" + str(OUT / "tmp"), "build",
               "-c", "opt", "--jobs=4", "--local_ram_resources=4096",
               "--repository_cache=" + str(OUT / "repository-cache"),
               "--disk_cache=" + str(OUT / "disk-cache"),
               "lyra/cli_example:encoder_main", "lyra/cli_example:decoder_main"]
    if args.python:
        command.insert(-2, "--repo_env=PYTHON_BIN_PATH=" + str(args.python.resolve()))
    if args.glog_060:
        glog = OUT / "glog-0.6.0"
        actual_glog = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=glog,
                                              text=True).strip()
        if actual_glog != "b33e3bad4c46c8a6345525fd822af355e5ef9446":
            raise ValueError("unexpected glog commit")
        command.insert(-2, "--override_repository=com_google_glog=" + str(glog))
    raise SystemExit(run(command, args.label, source, args.timeout, env))


if __name__ == "__main__":
    main()
