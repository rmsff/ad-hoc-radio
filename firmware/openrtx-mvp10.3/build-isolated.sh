#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUNDLE_DIR="${ROOT_DIR}/firmware/openrtx-mvp10.3"
LOCK_FILE="${BUNDLE_DIR}/dependency-lock.json"
BASE_SOURCE="${OPENRTX_BASE_SOURCE:-/tmp/adhr-mvp10-openrtx/OpenRTX}"
CACHE_ID="${OPENRTX_DEPENDENCY_CACHE_ID:-default}"
CACHE_BASE="/tmp/adhr-mvp10-openrtx/dependency-caches"
RUN_ID="${1:?usage: build-isolated.sh RUN_ID}"
RUN_BASE="/tmp/adhr-mvp10-openrtx/runs"
EXPECTED_SCRATCH_BASE="$(cd /tmp && pwd -P)/adhr-mvp10-openrtx"
IMAGE="adhr-openrtx-builder:ubuntu24-miosix9.2"
IMAGE_ID="sha256:6a25272b232219b3b7201696125c32a54e0c6d018bca823f3a44a87a8f70a93e"
SOURCE_DATE_EPOCH="1777626376"
BUILD_DIR="build_mvp10_3"

for identifier in "${RUN_ID}" "${CACHE_ID}"; do
  if [[ ! "${identifier}" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]]; then
    echo "invalid scratch identifier: ${identifier}" >&2
    exit 1
  fi
done

mkdir -p "${RUN_BASE}" "${CACHE_BASE}"
RUN_BASE="$(cd "${RUN_BASE}" && pwd -P)"
CACHE_BASE="$(cd "${CACHE_BASE}" && pwd -P)"
if [[ "${RUN_BASE}" != "${EXPECTED_SCRATCH_BASE}/runs" ]] ||
   [[ "${CACHE_BASE}" != "${EXPECTED_SCRATCH_BASE}/dependency-caches" ]]; then
  echo "unsafe scratch base" >&2
  exit 1
fi

RUN_ROOT="${RUN_BASE}/${RUN_ID}"
DEPENDENCY_CACHE="${CACHE_BASE}/${CACHE_ID}"
SOURCE_DIR="${RUN_ROOT}/source"
OUTPUT_DIR="${RUN_ROOT}/output"

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
import re
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    lock = json.load(source)
seen = set()
for dependency in lock["dependencies"]:
    name = dependency["name"]
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", name):
        raise SystemExit(f"unsafe dependency name: {name}")
    if name in seen:
        raise SystemExit(f"duplicate dependency name: {name}")
    seen.add(name)
    print("\t".join((
        name,
        dependency["commit"],
        dependency["content_sha256"],
    )))
PY
}

verify_dependency()
{
  local name="$1"
  local commit="$2"
  local expected_hash="$3"
  local directory="${DEPENDENCY_CACHE}/${name}"

  if [[ ! -d "${directory}/.git" ]]; then
    echo "missing dependency cache entry: ${name}" >&2
    exit 1
  fi
  if [[ "$(git -C "${directory}" rev-parse HEAD)" != "${commit}" ]]; then
    echo "dependency commit mismatch: ${name}" >&2
    exit 1
  fi
  if [[ "$(content_hash "${directory}")" != "${expected_hash}" ]]; then
    echo "dependency content mismatch: ${name}" >&2
    exit 1
  fi
}

if [[ ! -d "${BASE_SOURCE}/.git" ]]; then
  echo "missing pinned OpenRTX checkout: ${BASE_SOURCE}" >&2
  exit 1
fi

COMMIT="$(python3 -c \
  'import json,sys; print(json.load(open(sys.argv[1]))["openrtx_commit"])' \
  "${LOCK_FILE}")"
if [[ "$(git -C "${BASE_SOURCE}" rev-parse HEAD)" != "${COMMIT}" ]]; then
  echo "OpenRTX base source commit mismatch" >&2
  exit 1
fi
if [[ -n "$(git -C "${BASE_SOURCE}" status --short)" ]]; then
  echo "OpenRTX base source must be clean" >&2
  exit 1
fi
if [[ "$(docker image inspect "${IMAGE}" --format '{{.Id}}')" != "${IMAGE_ID}" ]]; then
  echo "builder image digest mismatch" >&2
  exit 1
fi

while IFS=$'\t' read -r name commit expected_hash; do
  verify_dependency "${name}" "${commit}" "${expected_hash}"
done < <(dependency_rows)

rm -rf "${RUN_ROOT}"
mkdir -p "${RUN_ROOT}" "${OUTPUT_DIR}"
git clone --quiet --no-local "${BASE_SOURCE}" "${SOURCE_DIR}"
git -C "${SOURCE_DIR}" checkout --quiet --detach "${COMMIT}"

while IFS=$'\t' read -r name _ _; do
  cp -R "${DEPENDENCY_CACHE}/${name}" "${SOURCE_DIR}/subprojects/"
done < <(dependency_rows)

cp -R "${BUNDLE_DIR}/overlay/openrtx/." "${SOURCE_DIR}/openrtx/"
git -C "${SOURCE_DIR}" apply "${BUNDLE_DIR}/existing-files.patch"
git -C "${SOURCE_DIR}" add -N \
  openrtx/include/rtx/adhr_rxonly.h \
  openrtx/include/rtx/adhr_tx_queue.h \
  openrtx/include/rtx/adhr_usb_rxonly.h \
  openrtx/src/rtx/adhr_rxonly.c \
  openrtx/src/rtx/adhr_tx_queue.c \
  openrtx/src/rtx/adhr_usb_rxonly.c
git -C "${SOURCE_DIR}" diff --check
git -C "${SOURCE_DIR}" diff --binary > "${OUTPUT_DIR}/applied-source.patch"

docker run --rm --platform linux/amd64 \
  --network none \
  -e LANG=C \
  -e LC_ALL=C \
  -e PYTHONHASHSEED=0 \
  -e SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
  -e TZ=UTC \
  -e ZERO_AR_DATE=1 \
  -v "${SOURCE_DIR}:/src" \
  -v "${OUTPUT_DIR}:/evidence" \
  -w /src \
  "${IMAGE}" \
  bash -lc "set -euo pipefail
    meson setup --wrap-mode=nodownload --cross-file cross_cm4.txt '${BUILD_DIR}'
    meson compile -C '${BUILD_DIR}' openrtx_mduv3x0
    meson compile -C '${BUILD_DIR}' openrtx_mduv3x0_bin
    meson compile -C '${BUILD_DIR}' openrtx_mduv3x0_wrap
    cp '${BUILD_DIR}/openrtx_mduv3x0' /evidence/openrtx_mduv3x0.elf
    cp '${BUILD_DIR}/openrtx_mduv3x0_bin' /evidence/openrtx_mduv3x0.bin
    cp '${BUILD_DIR}/openrtx_mduv3x0_wrap' /evidence/openrtx_mduv3x0.wrap
    cp '${BUILD_DIR}/main.map' /evidence/openrtx_mduv3x0.map
    arm-miosix-eabi-size '${BUILD_DIR}/openrtx_mduv3x0' > /evidence/size.txt
    arm-miosix-eabi-size -A '${BUILD_DIR}/openrtx_mduv3x0' > /evidence/sections.txt
    arm-miosix-eabi-nm -C '${BUILD_DIR}/openrtx_mduv3x0' > /evidence/symbols.txt
    arm-miosix-eabi-objdump -d '${BUILD_DIR}/openrtx_mduv3x0' > /evidence/disassembly.txt
    {
      echo '[adhr_rxonly.c.o]'
      arm-miosix-eabi-nm -u \
        '${BUILD_DIR}/openrtx_mduv3x0.p/openrtx_src_rtx_adhr_rxonly.c.o'
      echo '[adhr_tx_queue.c.o]'
      arm-miosix-eabi-nm -u \
        '${BUILD_DIR}/openrtx_mduv3x0.p/openrtx_src_rtx_adhr_tx_queue.c.o'
      echo '[adhr_usb_rxonly.c.o]'
      arm-miosix-eabi-nm -u \
        '${BUILD_DIR}/openrtx_mduv3x0.p/openrtx_src_rtx_adhr_usb_rxonly.c.o'
    } > /evidence/adhr-undefined-symbols.txt
    sed -n '/<radio_enableTx>:/,+2p' /evidence/disassembly.txt \
      > /evidence/tx-guard-disassembly.txt
    sed -n '/<vcom_tryWriteBlock>:/,+16p' /evidence/disassembly.txt \
      > /evidence/usb-try-write-disassembly.txt
    strings '${BUILD_DIR}/openrtx_mduv3x0_bin' \
      | grep -E 'ADHR_RX_ONLY|ADHR1 (OK|EVENT|ERROR)' \
      > /evidence/adhr-protocol-strings.txt
    {
      meson --version
      arm-miosix-eabi-gcc --version | head -1
      arm-miosix-eabi-objdump --version | head -1
    } > /evidence/toolchain.txt
    cd /evidence
    sha256sum openrtx_mduv3x0.elf openrtx_mduv3x0.bin \
      openrtx_mduv3x0.wrap openrtx_mduv3x0.map size.txt sections.txt \
      symbols.txt disassembly.txt adhr-undefined-symbols.txt \
      tx-guard-disassembly.txt usb-try-write-disassembly.txt \
      adhr-protocol-strings.txt toolchain.txt applied-source.patch \
      > SHA256SUMS
  " 2>&1 | tee "${OUTPUT_DIR}/build.log"

set +e
rg -n -i 'codec2|m17|dmr|ambe' \
  "${BUNDLE_DIR}/overlay/openrtx" "${BUNDLE_DIR}/host" \
  > "${OUTPUT_DIR}/source-isolation.txt"
RG_STATUS=$?
set -e
case "${RG_STATUS}" in
  0)
    echo "ADHR boundary contains forbidden codec/backend coupling" >&2
    exit 1
    ;;
  1)
    printf '%s\n' \
      'no codec2, M17, DMR or AMBE imports/calls in ADHR boundary sources' \
      > "${OUTPUT_DIR}/source-isolation.txt"
    ;;
  *)
    echo "source-isolation scan failed with status ${RG_STATUS}" >&2
    exit 1
    ;;
esac
(
  cd "${ROOT_DIR}"
  find firmware/openrtx-mvp10.3 -type f \
    ! -path 'firmware/openrtx-mvp10.3/evidence/*' \
    ! -name 'build.log' -print0 \
    | env LANG=C LC_ALL=C LC_CTYPE=C sort -z \
    | xargs -0 shasum -a 256
) > "${OUTPUT_DIR}/bundle-source-hashes.txt"
sha256sum "${OUTPUT_DIR}/build.log" \
  "${OUTPUT_DIR}/bundle-source-hashes.txt" \
  "${OUTPUT_DIR}/source-isolation.txt" >> "${OUTPUT_DIR}/SHA256SUMS"

printf 'build output: %s\n' "${OUTPUT_DIR}"
