#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
set -euo pipefail

export LC_ALL=C
export LANG=C

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
PACKAGE="$ROOT/firmware/opengd77-mvp10.4"
OUT="$ROOT/.artifacts/opengd77-mvp10.4/host-tests"

mkdir -p "$OUT"

cc -std=c11 -Wall -Wextra -Werror -pedantic \
	-I"$PACKAGE/overlay/MDUV380_firmware/application/include" \
	"$PACKAGE/host/adhr_mode_host_test.c" \
	"$PACKAGE/overlay/MDUV380_firmware/application/source/adhr/adhr_mode.c" \
	-o "$OUT/adhr_mode_host_test"

"$OUT/adhr_mode_host_test"
