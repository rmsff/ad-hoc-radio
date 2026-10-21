/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 */
#include "adhr/adhr_mode.h"

#include <string.h>

static adhr_mode_state_t state = {
	.mode = ADHR_MODE_ANALOG_FM,
	.last_reason = ADHR_MODE_REASON_BOOT,
};

__attribute__((weak)) void adhrModeInvalidateRX(void)
{
}

static bool valid_mode(adhr_mode_t mode)
{
	return (mode == ADHR_MODE_ANALOG_FM) || (mode == ADHR_MODE_ADHR);
}

static bool valid_reason(adhr_mode_reason_t reason)
{
	return (reason >= ADHR_MODE_REASON_BOOT) &&
		(reason <= ADHR_MODE_REASON_TIMEOUT);
}

static void reset_domains(void)
{
	state.codec_generation = state.transition_sequence;
	state.squelch_generation = state.transition_sequence;
	state.call_generation = state.transition_sequence;
	state.timing_generation = state.transition_sequence;
	state.rx_generation = state.transition_sequence;
	state.tx_generation = state.transition_sequence;
}

void adhrModeInitialize(void)
{
	memset(&state, 0, sizeof(state));
	state.mode = ADHR_MODE_ANALOG_FM;
	state.last_reason = ADHR_MODE_REASON_BOOT;
	adhrModeInvalidateRX();
}

bool adhrModeTransition(adhr_mode_t target, adhr_mode_reason_t reason)
{
	if (!valid_mode(target))
	{
		state.last_error = ADHR_MODE_ERROR_INVALID_TARGET;
		return false;
	}
	if (!valid_reason(reason))
	{
		state.last_error = ADHR_MODE_ERROR_INVALID_REASON;
		return false;
	}

	bool changed = state.mode != target;
	state.transition_sequence++;
	state.mode = target;
	state.last_reason = reason;
	state.last_error = ADHR_MODE_ERROR_NONE;
	reset_domains();
	if (changed)
	{
		adhrModeInvalidateRX();
	}
	return true;
}

void adhrModeReset(adhr_mode_reason_t reason)
{
	if (!valid_reason(reason))
	{
		state.last_error = ADHR_MODE_ERROR_INVALID_REASON;
		return;
	}

	state.transition_sequence++;
	state.last_reason = reason;
	state.last_error = ADHR_MODE_ERROR_NONE;
	reset_domains();
	adhrModeInvalidateRX();
}

adhr_mode_t adhrModeCurrent(void)
{
	return state.mode;
}

const adhr_mode_state_t *adhrModeState(void)
{
	return &state;
}

const char *adhrModeLabel(adhr_mode_t mode)
{
	switch (mode)
	{
		case ADHR_MODE_ANALOG_FM:
			return "ANALOG FM";
		case ADHR_MODE_ADHR:
			return "ADHR";
		default:
			return "INVALID";
	}
}

bool adhrModeTXAllowed(void)
{
	return false;
}
