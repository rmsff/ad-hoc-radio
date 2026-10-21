/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 */
#include "adhr/adhr_l2_envelope.h"

#include <assert.h>
#include <stdio.h>
#include <string.h>

static const uint8_t valid_wire[ADHR_L2_WIRE_SIZE] = {
	0xA7, 0x01, 0x16, 0x00, 0xFC,
	0x01, 0x10, 0x00, 0x02, 0x03, 0x14, 0xCB, 0xF0, 0xB6, 0x6E, 0xB7,
	0x10, 0xF7, 0xEB, 0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80,
};

static void expect_reason(uint8_t *wire, size_t length, adhr_rx_reason_t reason)
{
	adhrL2Initialize();
	adhr_rx_result_t result = adhrL2Process(ADHR_MODE_ADHR, wire, length, 7U);
	assert(result.consumed);
	assert(result.event_queued);
	assert(result.reason == reason);
	adhr_rx_event_t event;
	assert(adhrL2PopEvent(&event));
	assert(event.kind == ADHR_RX_EVENT_REJECTED);
	assert(event.reason == reason);
	assert(event.payload_length == 0U);
}

static void test_valid_and_deterministic_event(void)
{
	uint8_t first[ADHR_L2_EVENT_WIRE_SIZE];
	uint8_t second[ADHR_L2_EVENT_WIRE_SIZE];
	for (int run = 0; run < 2; run++)
	{
		adhrL2Initialize();
		adhr_rx_result_t result = adhrL2Process(ADHR_MODE_ADHR, valid_wire, sizeof(valid_wire), 0x01020304U);
		assert(result.consumed && result.event_queued);
		assert(result.reason == ADHR_RX_REASON_ACCEPTED);
		assert(result.sequence == 1U);
		adhr_rx_event_t event;
		assert(adhrL2PopEvent(&event));
		assert(event.kind == ADHR_RX_EVENT_FRAME);
		assert(event.payload_length == ADHR_L2_PAYLOAD_SIZE);
		assert(memcmp(event.payload, &valid_wire[5], ADHR_L2_PAYLOAD_SIZE) == 0);
		assert(adhrL2SerializeEvent(&event, run == 0 ? first : second, ADHR_L2_EVENT_WIRE_SIZE));
	}
	assert(memcmp(first, second, sizeof(first)) == 0);
	assert(first[0] == 1U && first[1] == ADHR_RX_EVENT_FRAME && first[3] == ADHR_L2_PAYLOAD_SIZE);
}

static void test_malformed(void)
{
	uint8_t wire[ADHR_L2_WIRE_SIZE + 1U];
	memcpy(wire, valid_wire, sizeof(valid_wire));
	wire[0] = 0;
	expect_reason(wire, ADHR_L2_WIRE_SIZE, ADHR_RX_REASON_WRONG_DISCRIMINATOR);
	memcpy(wire, valid_wire, sizeof(valid_wire));
	wire[1] = 2;
	expect_reason(wire, ADHR_L2_WIRE_SIZE, ADHR_RX_REASON_WRONG_VERSION);
	memcpy(wire, valid_wire, sizeof(valid_wire));
	wire[2] = 21;
	expect_reason(wire, ADHR_L2_WIRE_SIZE, ADHR_RX_REASON_WRONG_LENGTH);
	memcpy(wire, valid_wire, sizeof(valid_wire));
	wire[3] = 1;
	expect_reason(wire, ADHR_L2_WIRE_SIZE, ADHR_RX_REASON_RESERVED);
	memcpy(wire, valid_wire, sizeof(valid_wire));
	wire[4] ^= 1U;
	expect_reason(wire, ADHR_L2_WIRE_SIZE, ADHR_RX_REASON_INTEGRITY);
	memcpy(wire, valid_wire, sizeof(valid_wire));
	expect_reason(wire, ADHR_L2_WIRE_SIZE - 1U, ADHR_RX_REASON_TRUNCATED);
	wire[ADHR_L2_WIRE_SIZE] = 0;
	expect_reason(wire, ADHR_L2_WIRE_SIZE + 1U, ADHR_RX_REASON_OVERSIZE);
}

static void test_wrong_mode_and_pressure(void)
{
	adhrL2Initialize();
	adhr_rx_result_t wrong_mode = adhrL2Process(ADHR_MODE_ANALOG_FM, valid_wire, sizeof(valid_wire), 1U);
	assert(!wrong_mode.consumed && !wrong_mode.event_queued);
	assert(wrong_mode.reason == ADHR_RX_REASON_WRONG_MODE);
	assert(adhrL2State()->sequence == 0U && adhrL2State()->pending_count == 0U);

	for (uint32_t index = 0; index < ADHR_L2_QUEUE_CAPACITY; index++)
	{
		assert(adhrL2Process(ADHR_MODE_ADHR, valid_wire, sizeof(valid_wire), index).event_queued);
	}
	adhr_rx_result_t pressure = adhrL2Process(ADHR_MODE_ADHR, valid_wire, sizeof(valid_wire), 9U);
	assert(pressure.consumed && !pressure.event_queued);
	assert(pressure.reason == ADHR_RX_REASON_QUEUE_PRESSURE);
	assert(adhrL2State()->pending_count == ADHR_L2_QUEUE_CAPACITY);
	assert(adhrL2State()->pressure_count == 1U);
}

static void test_lifecycle(void)
{
	const adhr_rx_reason_t reasons[] = {
		ADHR_RX_REASON_TIMEOUT,
		ADHR_RX_REASON_RESET,
		ADHR_RX_REASON_DISCONNECT,
	};
	for (size_t index = 0; index < sizeof(reasons) / sizeof(reasons[0]); index++)
	{
		adhrL2Initialize();
		(void)adhrL2Process(ADHR_MODE_ADHR, valid_wire, sizeof(valid_wire), 1U);
		adhr_rx_result_t lifecycle = adhrL2Lifecycle(reasons[index], 2U);
		assert(lifecycle.consumed && lifecycle.event_queued);
		assert(lifecycle.reason == reasons[index]);
		assert(adhrL2State()->pending_count == 1U);
		assert(adhrL2State()->generation == 1U);
		adhr_rx_event_t event;
		assert(adhrL2PopEvent(&event));
		assert(event.kind == ADHR_RX_EVENT_LIFECYCLE && event.reason == reasons[index]);
	}
	assert(!adhrL2TXAllowed());
}

static void test_mode_queue_boundary(void)
{
	for (unsigned count = 1; count <= ADHR_L2_QUEUE_CAPACITY; count++)
	{
		adhrModeInitialize();
		adhrL2Initialize();
		assert(adhrModeTransition(ADHR_MODE_ADHR, ADHR_MODE_REASON_USER_SELECT));
		for (unsigned i = 0; i < count; i++)
		{
			assert(adhrL2Process(adhrModeCurrent(), valid_wire, sizeof(valid_wire), i).event_queued);
		}
		uint32_t sequence = adhrL2State()->sequence;
		uint32_t generation = adhrL2State()->generation;
		assert(adhrModeTransition(ADHR_MODE_ADHR, ADHR_MODE_REASON_USER_SELECT));
		assert(adhrL2State()->pending_count == count);
		assert(adhrL2State()->generation == generation);
		assert(!adhrModeTransition((adhr_mode_t)99, ADHR_MODE_REASON_USER_SELECT));
		assert(adhrL2State()->pending_count == count);
		assert(adhrModeTransition(ADHR_MODE_ANALOG_FM, ADHR_MODE_REASON_USER_SELECT));
		assert(strcmp(adhrModeLabel(adhrModeCurrent()), "ANALOG FM") == 0);
		assert(adhrL2State()->pending_count == 0);
		assert(adhrL2State()->generation == generation + 1U);
		assert(adhrL2State()->sequence == sequence);
		adhr_rx_event_t event;
		assert(!adhrL2PopEvent(&event));
		assert(!adhrL2Process(adhrModeCurrent(), valid_wire, sizeof(valid_wire), 10).consumed);
		assert(adhrL2State()->pending_count == 0);
		assert(adhrModeTransition(ADHR_MODE_ADHR, ADHR_MODE_REASON_USER_SELECT));
		assert(strcmp(adhrModeLabel(adhrModeCurrent()), "ADHR") == 0);
		assert(!adhrL2PopEvent(&event));
		assert(adhrL2Process(adhrModeCurrent(), valid_wire, sizeof(valid_wire), 11).sequence == sequence + 1U);
		adhrModeReset(ADHR_MODE_REASON_TIMEOUT);
		assert(adhrL2State()->pending_count == 0);
		assert(!adhrL2PopEvent(&event));
		assert(adhrL2Process(adhrModeCurrent(), valid_wire, sizeof(valid_wire), 12).event_queued);
		adhrModeInitialize();
		assert(adhrL2State()->pending_count == 0);
		assert(!adhrL2PopEvent(&event));
	}
}

int main(void)
{
	test_valid_and_deterministic_event();
	test_malformed();
	test_wrong_mode_and_pressure();
	test_lifecycle();
	test_mode_queue_boundary();
	puts("adhr_l2_host_test: PASS");
	return 0;
}
