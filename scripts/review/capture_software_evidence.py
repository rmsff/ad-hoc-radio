#!/usr/bin/env python3
"""Create a source-and-report hash inventory; never copy file contents/secrets."""

import argparse
import hashlib
import json
from pathlib import Path
import subprocess


ROOTS = {"cmd", "internal", "firmware", "configs", "scripts", "docs"}
SUFFIXES = {".go", ".c", ".h", ".py", ".sh", ".json", ".md", ".yaml", ".yml", ".toml", ".patch", ".ld", ".mk", ".s", ".S"}
BUILD_NAMES = {"Makefile", "Dockerfile", "CMakeLists.txt"}


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def capture(root, reports, destination):
    root = root.resolve()
    paths = subprocess.check_output(
        ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
        cwd=root,
    ).decode().split("\0")
    rows = []
    for name in sorted(set(paths)):
        p = Path(name)
        if not name or (name not in {"go.mod", "go.sum", "AGENTS.md"} and
                        (p.parts[0] not in ROOTS or (p.suffix not in SUFFIXES and p.name not in BUILD_NAMES))):
            continue
        if p.parts[:3] == ("docs", "reviews", "evidence") or name.endswith("-checkpoint.md"):
            continue
        source = root / p
        if source.resolve() == destination.resolve() or not source.is_file() or source.is_symlink():
            continue
        rows.append({"path": name, "sha256": digest(source)})
    outputs = []
    for path in reports:
        path = path.resolve()
        outputs.append({"path": str(path.relative_to(root)), "sha256": digest(path)})
    result = {
        "version": 1,
        "evidence_grade": "software_only_uncommitted_worktree",
        "head": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip(),
        "sources": rows,
        "reports": sorted(outputs, key=lambda row: row["path"]),
        "limits": ["Hashes bind bytes, not correctness or physical acceptance.",
                   "Historical files are inventoried as inputs, never upgraded to current test proof.",
                   "Firmware build closures are verified separately by revision receipts and IT088/089/090.",
                   "Evidence-directory artifacts and mutable checkpoint logs are excluded from source hashing."],
    }
    encoded = json.dumps(result, sort_keys=True, separators=(",", ":")).encode()
    result["sha256"] = hashlib.sha256(encoded).hexdigest()
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    parser.add_argument("--report", type=Path, action="append", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    result = capture(args.root, args.report, args.output)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"sources": len(result["sources"]), "reports": len(result["reports"]), "sha256": result["sha256"]}))


if __name__ == "__main__":
    main()
