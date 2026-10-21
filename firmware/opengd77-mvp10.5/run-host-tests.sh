#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
set -euo pipefail

export LC_ALL=C
export LANG=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
BASE="$ROOT/firmware/opengd77-mvp10.4"
PACKAGE="$ROOT/firmware/opengd77-mvp10.5"
OUT="$ROOT/.artifacts/opengd77-mvp10.5/host-tests"

mkdir -p "$OUT"

cc -std=c11 -Wall -Wextra -Werror -pedantic \
	-I"$BASE/overlay/MDUV380_firmware/application/include" \
	-I"$PACKAGE/overlay/MDUV380_firmware/application/include" \
	"$PACKAGE/host/adhr_l2_host_test.c" \
	"$BASE/overlay/MDUV380_firmware/application/source/adhr/adhr_mode.c" \
	"$PACKAGE/overlay/MDUV380_firmware/application/source/adhr/adhr_l2_envelope.c" \
	-o "$OUT/adhr_l2_host_test"

"$OUT/adhr_l2_host_test"
