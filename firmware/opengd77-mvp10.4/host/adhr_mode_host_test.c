/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 */
#include "adhr/adhr_mode.h"

#include <assert.h>
#include <stdio.h>
#include <string.h>

static void assert_all_generations(uint32_t expected)
{
	const adhr_mode_state_t *state = adhrModeState();
	assert(state->codec_generation == expected);
	assert(state->squelch_generation == expected);
	assert(state->call_generation == expected);
	assert(state->timing_generation == expected);
	assert(state->rx_generation == expected);
	assert(state->tx_generation == expected);
}

int main(void)
{
	adhrModeInitialize();
	assert(adhrModeCurrent() == ADHR_MODE_ANALOG_FM);
	assert(strcmp(adhrModeLabel(ADHR_MODE_ANALOG_FM), "ANALOG FM") == 0);
	assert(strcmp(adhrModeLabel(ADHR_MODE_ADHR), "ADHR") == 0);
	assert(!adhrModeTXAllowed());
	assert(adhrModeState()->transition_sequence == 0U);
	assert_all_generations(0U);

	assert(adhrModeTransition(ADHR_MODE_ADHR, ADHR_MODE_REASON_USER_SELECT));
	assert(adhrModeCurrent() == ADHR_MODE_ADHR);
	assert(adhrModeState()->last_reason == ADHR_MODE_REASON_USER_SELECT);
	assert(adhrModeState()->transition_sequence == 1U);
	assert_all_generations(1U);

	assert(adhrModeTransition(ADHR_MODE_ANALOG_FM, ADHR_MODE_REASON_USER_SELECT));
	assert(adhrModeCurrent() == ADHR_MODE_ANALOG_FM);
	assert(adhrModeState()->transition_sequence == 2U);
	assert_all_generations(2U);

	assert(!adhrModeTransition(ADHR_MODE_INVALID, ADHR_MODE_REASON_USER_SELECT));
	assert(adhrModeCurrent() == ADHR_MODE_ANALOG_FM);
	assert(adhrModeState()->transition_sequence == 2U);
	assert(adhrModeState()->last_error == ADHR_MODE_ERROR_INVALID_TARGET);

	adhrModeReset(ADHR_MODE_REASON_DISCONNECT);
	assert(adhrModeCurrent() == ADHR_MODE_ANALOG_FM);
	assert(adhrModeState()->transition_sequence == 3U);
	assert(adhrModeState()->last_reason == ADHR_MODE_REASON_DISCONNECT);
	assert_all_generations(3U);
	assert(!adhrModeTXAllowed());

	puts("adhr_mode_host_test: PASS");
	return 0;
}
