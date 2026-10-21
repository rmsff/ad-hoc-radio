/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 */
#include "adhr/adhr_usb_bench.h"

static bool set_mode(adhr_mode_t mode)
{
	return adhrModeTransition(mode, ADHR_MODE_REASON_USER_SELECT);
}

static uint8_t tx_flags(void) { return 0; }
static uint32_t tick(void) { return 42; }

size_t host_exchange(const uint8_t *request, size_t length, uint8_t *response, size_t capacity)
{
	const adhr_usb_bench_ops_t ops = { .set_mode = set_mode, .tx_flags = tx_flags, .tick = tick };
	return adhrUsbBenchHandle(request, length, response, capacity, &ops);
}
