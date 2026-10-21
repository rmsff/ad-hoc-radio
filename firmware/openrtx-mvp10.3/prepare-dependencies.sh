#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUNDLE_DIR="${ROOT_DIR}/firmware/openrtx-mvp10.3"
LOCK_FILE="${BUNDLE_DIR}/dependency-lock.json"
BASE_SOURCE="${OPENRTX_BASE_SOURCE:-/tmp/adhr-mvp10-openrtx/OpenRTX}"
CACHE_ID="${1:-default}"
CACHE_BASE="/tmp/adhr-mvp10-openrtx/dependency-caches"
EXPECTED_CACHE_BASE="$(cd /tmp && pwd -P)/adhr-mvp10-openrtx/dependency-caches"

if [[ ! "${CACHE_ID}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
  echo "invalid dependency cache id: ${CACHE_ID}" >&2
  exit 1
fi

mkdir -p "${CACHE_BASE}"
CACHE_BASE="$(cd "${CACHE_BASE}" && pwd -P)"
if [[ "${CACHE_BASE}" != "${EXPECTED_CACHE_BASE}" ]]; then
  echo "unsafe dependency cache base: ${CACHE_BASE}" >&2
  exit 1
fi
CACHE_ROOT="${CACHE_BASE}/${CACHE_ID}"

content_hash()
{
  local directory="$1"
  (
    export LANG=C LC_ALL=C LC_CTYPE=C
    cd "${directory}"
    find . -type f ! -path './.git/*' -print0 \
      | sort -z \
      | xargs -0 shasum -a 256
  ) | env LANG=C LC_ALL=C LC_CTYPE=C shasum -a 256 | awk '{print $1}'
}

dependency_rows()
{
  python3 - "${LOCK_FILE}" <<'PY'
import json
from pathlib import PurePosixPath
import re
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    lock = json.load(source)
seen = set()
for dependency in lock["dependencies"]:
    name = dependency["name"]
    overlay = PurePosixPath(dependency["package_overlay"])
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", name):
        raise SystemExit(f"unsafe dependency name: {name}")
    if name in seen:
        raise SystemExit(f"duplicate dependency name: {name}")
    if overlay.is_absolute() or ".." in overlay.parts:
        raise SystemExit(f"unsafe package overlay: {overlay}")
    if overlay.parts[:2] != ("subprojects", "packagefiles"):
        raise SystemExit(f"unsafe package overlay: {overlay}")
    seen.add(name)
    print("\t".join((
        name,
        dependency["repo"],
        dependency["commit"],
        dependency["content_sha256"],
        str(overlay),
    )))
PY
}

prepare()
{
  local name="$1"
  local repo="$2"
  local commit="$3"
  local expected_hash="$4"
  local package_overlay="$5"
  local overlay="${BASE_SOURCE}/${package_overlay}"
  local directory="${CACHE_ROOT}/${name}"

  case "${package_overlay}" in
    subprojects/packagefiles/*) ;;
    *)
      echo "unsafe package overlay: ${package_overlay}" >&2
      exit 1
      ;;
  esac

  git clone --quiet --no-checkout "${repo}" "${directory}"
  git -C "${directory}" checkout --quiet --detach "${commit}"
  cp -R "${overlay}/." "${directory}/"

  if [[ "$(content_hash "${directory}")" != "${expected_hash}" ]]; then
    echo "dependency content mismatch: ${name}" >&2
    exit 1
  fi
}

if [[ ! -d "${BASE_SOURCE}/.git" ]]; then
  echo "missing pinned OpenRTX checkout: ${BASE_SOURCE}" >&2
  exit 1
fi

EXPECTED_COMMIT="$(python3 -c \
  'import json,sys; print(json.load(open(sys.argv[1]))["openrtx_commit"])' \
  "${LOCK_FILE}")"
if [[ "$(git -C "${BASE_SOURCE}" rev-parse HEAD)" != "${EXPECTED_COMMIT}" ]]; then
  echo "OpenRTX base source commit does not match dependency lock" >&2
  exit 1
fi

rm -rf "${CACHE_ROOT}"
mkdir -p "${CACHE_ROOT}"

while IFS=$'\t' read -r name repo commit expected_hash package_overlay; do
  prepare "${name}" "${repo}" "${commit}" "${expected_hash}" \
    "${package_overlay}"
done < <(dependency_rows)

printf 'dependency cache: %s\n' "${CACHE_ROOT}"
