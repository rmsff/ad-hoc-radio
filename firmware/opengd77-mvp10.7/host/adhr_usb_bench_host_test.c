/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 */
#include "adhr/adhr_usb_bench.h"

#include <assert.h>
#include <stdio.h>
#include <string.h>

static const uint8_t valid_wire[ADHR_L2_WIRE_SIZE] = {
	0xA7, 0x01, 0x16, 0x00, 0xFC,
	0x01, 0x10, 0x00, 0x02, 0x03, 0x14, 0xCB, 0xF0, 0xB6, 0x6E, 0xB7,
	0x10, 0xF7, 0xEB, 0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80,
};

static uint8_t tx_flags;
static uint32_t tick_value = 0x01020304U;

static bool set_mode(adhr_mode_t target)
{
	return adhrModeTransition(target, ADHR_MODE_REASON_USER_SELECT);
}

static uint8_t get_tx_flags(void)
{
	return tx_flags;
}

static uint32_t get_tick(void)
{
	return tick_value++;
}

static const adhr_usb_bench_ops_t ops = {
	.set_mode = set_mode,
	.tx_flags = get_tx_flags,
	.tick = get_tick,
};

static uint32_t get_u32_be(const uint8_t *input)
{
	return ((uint32_t)input[0] << 24U) | ((uint32_t)input[1] << 16U) |
		((uint32_t)input[2] << 8U) | input[3];
}

static size_t request(uint8_t command, const uint8_t *payload, uint8_t payload_length, uint8_t *response)
{
	uint8_t wire[ADHR_USB_BENCH_MAX_REQUEST_SIZE] = {
		ADHR_USB_BENCH_MAGIC,
		ADHR_USB_BENCH_VERSION,
		command,
		payload_length,
	};
	if (payload_length > 0U)
	{
		memcpy(&wire[ADHR_USB_BENCH_REQUEST_HEADER_SIZE], payload, payload_length);
	}
	return adhrUsbBenchHandle(wire, ADHR_USB_BENCH_REQUEST_HEADER_SIZE + payload_length,
		response, ADHR_USB_BENCH_MAX_RESPONSE_SIZE, &ops);
}

static void expect_header(const uint8_t *response, size_t length, uint8_t command,
	adhr_usb_bench_status_t status, uint8_t payload_length)
{
	assert(length == ADHR_USB_BENCH_RESPONSE_HEADER_SIZE + payload_length);
	assert(response[0] == ADHR_USB_BENCH_MAGIC);
	assert(response[1] == ADHR_USB_BENCH_VERSION);
	assert(response[2] == command);
	assert(response[3] == status);
	assert(response[4] == payload_length);
}

static void reset_state(void)
{
	adhrModeInitialize();
	adhrL2Initialize();
	tx_flags = 0U;
	tick_value = 0x01020304U;
}

static void test_identity_and_status(void)
{
	uint8_t response[ADHR_USB_BENCH_MAX_RESPONSE_SIZE];
	reset_state();
	size_t length = request(ADHR_USB_BENCH_COMMAND_IDENTITY, NULL, 0U, response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_IDENTITY, ADHR_USB_BENCH_STATUS_OK, 8U);
	assert(response[5] == ADHR_USB_BENCH_VERSION);
	assert(response[7] == ADHR_L2_WIRE_SIZE);
	assert(response[8] == ADHR_L2_EVENT_WIRE_SIZE);
	assert((response[10] & ADHR_USB_BENCH_CAP_RX_ONLY) != 0U);

	length = request(ADHR_USB_BENCH_COMMAND_STATUS, NULL, 0U, response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_STATUS, ADHR_USB_BENCH_STATUS_OK,
		ADHR_USB_BENCH_STATUS_PAYLOAD_SIZE);
	assert(response[5] == ADHR_MODE_ANALOG_FM);
	assert(response[8] == 0U && response[9] == 0U && response[10] == 0U);
	assert(get_u32_be(&response[13]) == 0U);
}

static void test_mode_inject_and_event(void)
{
	uint8_t response[ADHR_USB_BENCH_MAX_RESPONSE_SIZE];
	reset_state();
	uint8_t mode = ADHR_MODE_ADHR;
	size_t length = request(ADHR_USB_BENCH_COMMAND_SET_MODE, &mode, 1U, response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_SET_MODE, ADHR_USB_BENCH_STATUS_OK, 1U);
	assert(response[5] == ADHR_MODE_ADHR);

	length = request(ADHR_USB_BENCH_COMMAND_INJECT_L2, valid_wire, sizeof(valid_wire), response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_INJECT_L2, ADHR_USB_BENCH_STATUS_OK, 8U);
	assert(response[5] == 1U && response[6] == 1U && response[7] == ADHR_RX_REASON_ACCEPTED);
	assert(get_u32_be(&response[9]) == 1U);

	length = request(ADHR_USB_BENCH_COMMAND_POP_EVENT, NULL, 0U, response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_POP_EVENT, ADHR_USB_BENCH_STATUS_OK,
		ADHR_L2_EVENT_WIRE_SIZE);
	assert(response[5] == 1U && response[6] == ADHR_RX_EVENT_FRAME);
	assert(response[7] == ADHR_RX_REASON_ACCEPTED && response[8] == ADHR_L2_PAYLOAD_SIZE);
	assert(memcmp(&response[17], &valid_wire[5], ADHR_L2_PAYLOAD_SIZE) == 0);

	length = request(ADHR_USB_BENCH_COMMAND_POP_EVENT, NULL, 0U, response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_POP_EVENT, ADHR_USB_BENCH_STATUS_NO_EVENT, 0U);
}

static void test_wrong_mode_lifecycle_and_reset(void)
{
	uint8_t response[ADHR_USB_BENCH_MAX_RESPONSE_SIZE];
	reset_state();
	size_t length = request(ADHR_USB_BENCH_COMMAND_INJECT_L2, valid_wire, sizeof(valid_wire), response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_INJECT_L2, ADHR_USB_BENCH_STATUS_OK, 8U);
	assert(response[5] == 0U && response[6] == 0U && response[7] == ADHR_RX_REASON_WRONG_MODE);

	uint8_t mode = ADHR_MODE_ADHR;
	(void)request(ADHR_USB_BENCH_COMMAND_SET_MODE, &mode, 1U, response);
	(void)request(ADHR_USB_BENCH_COMMAND_INJECT_L2, valid_wire, sizeof(valid_wire), response);
	uint8_t reason = ADHR_RX_REASON_TIMEOUT;
	uint32_t generation = adhrL2State()->generation;
	length = request(ADHR_USB_BENCH_COMMAND_LIFECYCLE, &reason, 1U, response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_LIFECYCLE, ADHR_USB_BENCH_STATUS_OK, 4U);
	assert(response[5] == 1U && response[6] == 1U && response[7] == ADHR_RX_REASON_TIMEOUT);
	assert(adhrL2State()->generation == generation + 1U && adhrL2State()->pending_count == 1U);

	length = request(ADHR_USB_BENCH_COMMAND_RESET, NULL, 0U, response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_RESET, ADHR_USB_BENCH_STATUS_OK, 0U);
	assert(adhrModeCurrent() == ADHR_MODE_ADHR);
	assert(adhrL2State()->pending_count == 0U && adhrL2State()->sequence == 0U);
}

static void test_malformed_and_tx_guard(void)
{
	uint8_t response[ADHR_USB_BENCH_MAX_RESPONSE_SIZE];
	reset_state();
	uint8_t malformed[4] = { ADHR_USB_BENCH_MAGIC, 2U, ADHR_USB_BENCH_COMMAND_STATUS, 0U };
	size_t length = adhrUsbBenchHandle(malformed, sizeof(malformed), response, sizeof(response), &ops);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_STATUS, ADHR_USB_BENCH_STATUS_BAD_VERSION, 0U);
	malformed[1] = ADHR_USB_BENCH_VERSION;
	malformed[3] = 1U;
	length = adhrUsbBenchHandle(malformed, sizeof(malformed), response, sizeof(response), &ops);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_STATUS, ADHR_USB_BENCH_STATUS_BAD_LENGTH, 0U);
	malformed[2] = 0x7FU;
	malformed[3] = 0U;
	length = adhrUsbBenchHandle(malformed, sizeof(malformed), response, sizeof(response), &ops);
	expect_header(response, length, 0x7FU, ADHR_USB_BENCH_STATUS_BAD_COMMAND, 0U);

	uint8_t mode = ADHR_MODE_ADHR;
	(void)request(ADHR_USB_BENCH_COMMAND_SET_MODE, &mode, 1U, response);
	tx_flags = ADHR_USB_BENCH_TX_PA_ENABLED;
	length = request(ADHR_USB_BENCH_COMMAND_INJECT_L2, valid_wire, sizeof(valid_wire), response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_INJECT_L2, ADHR_USB_BENCH_STATUS_TX_GUARD, 0U);
	assert(adhrL2State()->sequence == 0U && adhrL2State()->pending_count == 0U);

	length = request(ADHR_USB_BENCH_COMMAND_STATUS, NULL, 0U, response);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_STATUS, ADHR_USB_BENCH_STATUS_OK,
		ADHR_USB_BENCH_STATUS_PAYLOAD_SIZE);
	assert(response[8] == ADHR_USB_BENCH_TX_PA_ENABLED);

	tx_flags = 0U;
	uint8_t wire[ADHR_USB_BENCH_MAX_REQUEST_SIZE] = {
		ADHR_USB_BENCH_MAGIC,
		ADHR_USB_BENCH_VERSION,
		ADHR_USB_BENCH_COMMAND_INJECT_L2,
		ADHR_L2_WIRE_SIZE,
	};
	memcpy(&wire[ADHR_USB_BENCH_REQUEST_HEADER_SIZE], valid_wire, sizeof(valid_wire));
	length = adhrUsbBenchHandle(wire, sizeof(wire), response, sizeof(response), NULL);
	expect_header(response, length, ADHR_USB_BENCH_COMMAND_INJECT_L2, ADHR_USB_BENCH_STATUS_TX_GUARD, 0U);
	assert(adhrL2State()->sequence == 0U && adhrL2State()->pending_count == 0U);
}

int main(void)
{
	test_identity_and_status();
	test_mode_inject_and_event();
	test_wrong_mode_lifecycle_and_reset();
	test_malformed_and_tx_guard();
	puts("adhr_usb_bench_host_test: PASS");
	return 0;
}
