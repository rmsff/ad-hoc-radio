#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUNDLE_DIR="${ROOT_DIR}/firmware/openrtx-mvp10.3"
PARSER_OUTPUT="${TMPDIR:-/tmp}/adhr_rxonly_host_test"
USB_OUTPUT="${TMPDIR:-/tmp}/adhr_usb_rxonly_host_test"

cc -std=c17 -Wall -Wextra -Werror -pedantic \
  -I"${BUNDLE_DIR}/overlay/openrtx/include" \
  "${BUNDLE_DIR}/overlay/openrtx/src/rtx/adhr_rxonly.c" \
  "${BUNDLE_DIR}/overlay/openrtx/src/rtx/adhr_tx_queue.c" \
  "${BUNDLE_DIR}/host/adhr_rxonly_host_test.c" \
  -o "${PARSER_OUTPUT}"

"${PARSER_OUTPUT}"

cc -std=c17 -Wall -Wextra -Werror -pedantic \
  -I"${BUNDLE_DIR}/host/stubs" \
  -I"${BUNDLE_DIR}/overlay/openrtx/include" \
  "${BUNDLE_DIR}/overlay/openrtx/src/rtx/adhr_rxonly.c" \
  "${BUNDLE_DIR}/overlay/openrtx/src/rtx/adhr_tx_queue.c" \
  "${BUNDLE_DIR}/overlay/openrtx/src/rtx/adhr_usb_rxonly.c" \
  "${BUNDLE_DIR}/host/adhr_usb_rxonly_host_test.c" \
  -o "${USB_OUTPUT}"

"${USB_OUTPUT}"
