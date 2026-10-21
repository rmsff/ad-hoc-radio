/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 */
#include "adhr/adhr_l2_envelope.h"

#include <string.h>

static adhr_rx_event_t queue[ADHR_L2_QUEUE_CAPACITY];
static adhr_rx_state_t state;
static uint8_t queue_head;
static uint8_t queue_tail;

static uint8_t crc8_update(uint8_t crc, uint8_t value)
{
	crc ^= value;
	for (uint8_t bit = 0; bit < 8U; bit++)
	{
		crc = ((crc & 0x80U) != 0U) ? (uint8_t)((crc << 1U) ^ 0x07U) : (uint8_t)(crc << 1U);
	}
	return crc;
}

static uint8_t envelope_crc(const uint8_t *wire)
{
	uint8_t crc = 0;
	for (size_t index = 0; index < 4U; index++)
	{
		crc = crc8_update(crc, wire[index]);
	}
	for (size_t index = 5U; index < ADHR_L2_WIRE_SIZE; index++)
	{
		crc = crc8_update(crc, wire[index]);
	}
	return crc;
}

static adhr_rx_result_t result(bool consumed, bool queued, adhr_rx_reason_t reason, uint32_t sequence)
{
	adhr_rx_result_t value = {
		.consumed = consumed,
		.event_queued = queued,
		.reason = reason,
		.sequence = sequence,
	};
	return value;
}

static bool enqueue(adhr_rx_event_kind_t kind, adhr_rx_reason_t reason, const uint8_t *payload, uint8_t length, uint32_t observed_tick)
{
	if (state.pending_count >= ADHR_L2_QUEUE_CAPACITY)
	{
		return false;
	}
	adhr_rx_event_t *event = &queue[queue_tail];
	memset(event, 0, sizeof(*event));
	event->kind = kind;
	event->reason = reason;
	event->payload_length = length;
	event->sequence = state.sequence;
	event->observed_tick = observed_tick;
	if ((payload != NULL) && (length > 0U))
	{
		memcpy(event->payload, payload, length);
	}
	queue_tail = (uint8_t)((queue_tail + 1U) % ADHR_L2_QUEUE_CAPACITY);
	state.pending_count++;
	return true;
}

static adhr_rx_reason_t validate(const uint8_t *wire, size_t length)
{
	if ((wire == NULL) || (length < ADHR_L2_WIRE_SIZE))
	{
		return ADHR_RX_REASON_TRUNCATED;
	}
	if (length > ADHR_L2_WIRE_SIZE)
	{
		return ADHR_RX_REASON_OVERSIZE;
	}
	if (wire[0] != ADHR_L2_DISCRIMINATOR)
	{
		return ADHR_RX_REASON_WRONG_DISCRIMINATOR;
	}
	if (wire[1] != ADHR_L2_VERSION)
	{
		return ADHR_RX_REASON_WRONG_VERSION;
	}
	if (wire[2] != ADHR_L2_PAYLOAD_SIZE)
	{
		return ADHR_RX_REASON_WRONG_LENGTH;
	}
	if (wire[3] != 0U)
	{
		return ADHR_RX_REASON_RESERVED;
	}
	if (wire[4] != envelope_crc(wire))
	{
		return ADHR_RX_REASON_INTEGRITY;
	}
	return ADHR_RX_REASON_ACCEPTED;
}

void adhrL2Initialize(void)
{
	memset(queue, 0, sizeof(queue));
	memset(&state, 0, sizeof(state));
	queue_head = 0;
	queue_tail = 0;
}

void adhrModeInvalidateRX(void)
{
	uint32_t sequence = state.sequence;
	uint32_t generation = state.generation + 1U;
	adhrL2Initialize();
	state.sequence = sequence;
	state.generation = generation;
}

adhr_rx_result_t adhrL2Process(adhr_mode_t mode, const uint8_t *wire, size_t length, uint32_t observed_tick)
{
	if (mode != ADHR_MODE_ADHR)
	{
		return result(false, false, ADHR_RX_REASON_WRONG_MODE, 0);
	}
	state.sequence++;
	if (state.pending_count >= ADHR_L2_QUEUE_CAPACITY)
	{
		state.pressure_count++;
		return result(true, false, ADHR_RX_REASON_QUEUE_PRESSURE, state.sequence);
	}
	adhr_rx_reason_t reason = validate(wire, length);
	if (reason == ADHR_RX_REASON_ACCEPTED)
	{
		state.accepted_count++;
		(void)enqueue(ADHR_RX_EVENT_FRAME, reason, &wire[5], ADHR_L2_PAYLOAD_SIZE, observed_tick);
	}
	else
	{
		state.rejected_count++;
		(void)enqueue(ADHR_RX_EVENT_REJECTED, reason, NULL, 0, observed_tick);
	}
	return result(true, true, reason, state.sequence);
}

adhr_rx_result_t adhrL2Lifecycle(adhr_rx_reason_t reason, uint32_t observed_tick)
{
	if ((reason != ADHR_RX_REASON_TIMEOUT) && (reason != ADHR_RX_REASON_RESET) && (reason != ADHR_RX_REASON_DISCONNECT))
	{
		return result(false, false, ADHR_RX_REASON_INVALID_LIFECYCLE, 0);
	}
	adhr_mode_reason_t mode_reason = (reason == ADHR_RX_REASON_TIMEOUT) ? ADHR_MODE_REASON_TIMEOUT :
		((reason == ADHR_RX_REASON_DISCONNECT) ? ADHR_MODE_REASON_DISCONNECT : ADHR_MODE_REASON_RESET);
	adhrModeReset(mode_reason);
	state.sequence++;
	(void)enqueue(ADHR_RX_EVENT_LIFECYCLE, reason, NULL, 0, observed_tick);
	return result(true, true, reason, state.sequence);
}

bool adhrL2PopEvent(adhr_rx_event_t *event)
{
	if ((event == NULL) || (state.pending_count == 0U))
	{
		return false;
	}
	memcpy(event, &queue[queue_head], sizeof(*event));
	memset(&queue[queue_head], 0, sizeof(queue[queue_head]));
	queue_head = (uint8_t)((queue_head + 1U) % ADHR_L2_QUEUE_CAPACITY);
	state.pending_count--;
	return true;
}

const adhr_rx_state_t *adhrL2State(void)
{
	return &state;
}

bool adhrL2SerializeEvent(const adhr_rx_event_t *event, uint8_t *output, size_t output_size)
{
	if ((event == NULL) || (output == NULL) || (output_size != ADHR_L2_EVENT_WIRE_SIZE) ||
		(event->payload_length > ADHR_L2_PAYLOAD_SIZE))
	{
		return false;
	}
	memset(output, 0, output_size);
	output[0] = 1U;
	output[1] = (uint8_t)event->kind;
	output[2] = (uint8_t)event->reason;
	output[3] = event->payload_length;
	output[4] = (uint8_t)(event->sequence >> 24U);
	output[5] = (uint8_t)(event->sequence >> 16U);
	output[6] = (uint8_t)(event->sequence >> 8U);
	output[7] = (uint8_t)event->sequence;
	output[8] = (uint8_t)(event->observed_tick >> 24U);
	output[9] = (uint8_t)(event->observed_tick >> 16U);
	output[10] = (uint8_t)(event->observed_tick >> 8U);
	output[11] = (uint8_t)event->observed_tick;
	memcpy(&output[12], event->payload, ADHR_L2_PAYLOAD_SIZE);
	return true;
}

bool adhrL2TXAllowed(void)
{
	return false;
}
