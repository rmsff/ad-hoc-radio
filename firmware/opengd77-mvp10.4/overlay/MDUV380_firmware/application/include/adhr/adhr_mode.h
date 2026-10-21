/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 */
#ifndef ADHR_MODE_H
#define ADHR_MODE_H

#include <stdbool.h>
#include <stdint.h>

typedef enum
{
	ADHR_MODE_INVALID = 0,
	ADHR_MODE_ANALOG_FM = 1,
	ADHR_MODE_ADHR = 2
} adhr_mode_t;

typedef enum
{
	ADHR_MODE_REASON_BOOT = 1,
	ADHR_MODE_REASON_USER_SELECT = 2,
	ADHR_MODE_REASON_RADIO_REQUEST = 3,
	ADHR_MODE_REASON_DISCONNECT = 4,
	ADHR_MODE_REASON_RESET = 5,
	ADHR_MODE_REASON_TIMEOUT = 6
} adhr_mode_reason_t;

typedef enum
{
	ADHR_MODE_ERROR_NONE = 0,
	ADHR_MODE_ERROR_INVALID_TARGET = 1,
	ADHR_MODE_ERROR_INVALID_REASON = 2
} adhr_mode_error_t;

typedef struct
{
	adhr_mode_t mode;
	adhr_mode_reason_t last_reason;
	adhr_mode_error_t last_error;
	uint32_t transition_sequence;
	uint32_t codec_generation;
	uint32_t squelch_generation;
	uint32_t call_generation;
	uint32_t timing_generation;
	uint32_t rx_generation;
	uint32_t tx_generation;
} adhr_mode_state_t;

void adhrModeInitialize(void);
/* Synchronous RX invalidation; the base-only build supplies a weak no-op. */
void adhrModeInvalidateRX(void);
bool adhrModeTransition(adhr_mode_t target, adhr_mode_reason_t reason);
void adhrModeReset(adhr_mode_reason_t reason);
adhr_mode_t adhrModeCurrent(void);
const adhr_mode_state_t *adhrModeState(void);
const char *adhrModeLabel(adhr_mode_t mode);
bool adhrModeTXAllowed(void);

#endif
