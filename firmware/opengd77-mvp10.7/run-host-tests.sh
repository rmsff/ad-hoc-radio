#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
set -euo pipefail

export LC_ALL=C
export LANG=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
MODE="$ROOT/firmware/opengd77-mvp10.4"
L2="$ROOT/firmware/opengd77-mvp10.5"
PACKAGE="$ROOT/firmware/opengd77-mvp10.7"
OUT="$ROOT/.artifacts/opengd77-mvp10.7/host-tests"

mkdir -p "$OUT"

cc -std=c11 -Wall -Wextra -Werror -pedantic \
	-I"$MODE/overlay/MDUV380_firmware/application/include" \
	-I"$L2/overlay/MDUV380_firmware/application/include" \
	-I"$PACKAGE/overlay/MDUV380_firmware/application/include" \
	"$PACKAGE/host/adhr_usb_bench_host_test.c" \
	"$MODE/overlay/MDUV380_firmware/application/source/adhr/adhr_mode.c" \
	"$L2/overlay/MDUV380_firmware/application/source/adhr/adhr_l2_envelope.c" \
	"$PACKAGE/overlay/MDUV380_firmware/application/source/adhr/adhr_usb_bench.c" \
	-o "$OUT/adhr_usb_bench_host_test"

"$OUT/adhr_usb_bench_host_test"

python3 -B -m unittest discover -s "$PACKAGE/host" -p 'test_*.py'
