#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
IMAGE="${OPENRTX_BUILDER_IMAGE:-adhr-openrtx-builder:ubuntu24-miosix9.2}"
OPENRTX_TAG="${OPENRTX_TAG:-v0.4.4}"
OPENRTX_COMMIT="${OPENRTX_COMMIT:-801fc8dd313eb3ac6f83fd4d3b63ed895effdd56}"
OPENRTX_DIR="${OPENRTX_DIR:-/tmp/adhr-openrtx-isolated/OpenRTX}"
OPENRTX_REPO="${OPENRTX_REPO:-https://github.com/OpenRTX/OpenRTX.git}"
BUILD_DIR="${OPENRTX_BUILD_DIR:-build_cm4_adhr}"

mkdir -p "$(dirname "${OPENRTX_DIR}")"

if [[ ! -d "${OPENRTX_DIR}/.git" ]]; then
  git clone --branch "${OPENRTX_TAG}" --depth 1 "${OPENRTX_REPO}" "${OPENRTX_DIR}"
fi

git -C "${OPENRTX_DIR}" fetch --force --depth 1 origin "refs/tags/${OPENRTX_TAG}:refs/tags/${OPENRTX_TAG}"
git -C "${OPENRTX_DIR}" checkout --detach "${OPENRTX_TAG}"
ACTUAL_COMMIT="$(git -C "${OPENRTX_DIR}" rev-parse HEAD)"

if [[ "${ACTUAL_COMMIT}" != "${OPENRTX_COMMIT}" ]]; then
  echo "OpenRTX commit mismatch for ${OPENRTX_TAG}: expected ${OPENRTX_COMMIT}, got ${ACTUAL_COMMIT}" >&2
  exit 1
fi

docker build --platform linux/amd64 -t "${IMAGE}" "${ROOT_DIR}/tools/openrtx-build"

docker run --rm --platform linux/amd64 \
  -e OPENRTX_BUILD_DIR="${BUILD_DIR}" \
  -v "${OPENRTX_DIR}:/src" \
  -w /src \
  "${IMAGE}" \
  bash -lc "set -euo pipefail
    rm -rf \"\${OPENRTX_BUILD_DIR}\"
    meson setup --cross-file cross_cm4.txt \"\${OPENRTX_BUILD_DIR}\"
    meson compile -C \"\${OPENRTX_BUILD_DIR}\" openrtx_mduv3x0
    meson compile -C \"\${OPENRTX_BUILD_DIR}\" openrtx_mduv3x0_bin
    meson compile -C \"\${OPENRTX_BUILD_DIR}\" openrtx_mduv3x0_wrap
    arm-miosix-eabi-size \"\${OPENRTX_BUILD_DIR}/openrtx_mduv3x0\"
    arm-miosix-eabi-size -A \"\${OPENRTX_BUILD_DIR}/openrtx_mduv3x0\" | sed -n '1,40p'
    wc -c \"\${OPENRTX_BUILD_DIR}/openrtx_mduv3x0_bin\" \"\${OPENRTX_BUILD_DIR}/openrtx_mduv3x0_wrap\"
  "

echo "OpenRTX build artifacts are in ${OPENRTX_DIR}/${BUILD_DIR}"
