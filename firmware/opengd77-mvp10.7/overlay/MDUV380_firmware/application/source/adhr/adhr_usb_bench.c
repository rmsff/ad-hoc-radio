/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 */
#include "adhr/adhr_usb_bench.h"

#include <string.h>

static void put_u32_be(uint8_t *output, uint32_t value)
{
	output[0] = (uint8_t)(value >> 24U);
	output[1] = (uint8_t)(value >> 16U);
	output[2] = (uint8_t)(value >> 8U);
	output[3] = (uint8_t)value;
}

static size_t write_response(uint8_t command, adhr_usb_bench_status_t status,
	const uint8_t *payload, size_t payload_length, uint8_t *response, size_t response_capacity)
{
	if ((response == NULL) || (payload_length > UINT8_MAX) ||
		(response_capacity < (ADHR_USB_BENCH_RESPONSE_HEADER_SIZE + payload_length)))
	{
		return 0U;
	}
	response[0] = ADHR_USB_BENCH_MAGIC;
	response[1] = ADHR_USB_BENCH_VERSION;
	response[2] = command;
	response[3] = (uint8_t)status;
	response[4] = (uint8_t)payload_length;
	if ((payload != NULL) && (payload_length > 0U))
	{
		memcpy(&response[ADHR_USB_BENCH_RESPONSE_HEADER_SIZE], payload, payload_length);
	}
	return ADHR_USB_BENCH_RESPONSE_HEADER_SIZE + payload_length;
}

static uint32_t observed_tick(const adhr_usb_bench_ops_t *ops)
{
	return ((ops != NULL) && (ops->tick != NULL)) ? ops->tick() : 0U;
}

static uint8_t physical_tx_flags(const adhr_usb_bench_ops_t *ops)
{
	return ((ops != NULL) && (ops->tx_flags != NULL)) ? ops->tx_flags() : 0U;
}

static bool tx_guard_closed(const adhr_usb_bench_ops_t *ops)
{
	return (ops != NULL) && (ops->tx_flags != NULL) &&
		!adhrModeTXAllowed() && !adhrL2TXAllowed() && (physical_tx_flags(ops) == 0U);
}

static size_t identity_response(uint8_t command, uint8_t *response, size_t response_capacity)
{
	const uint8_t payload[8] = {
		ADHR_USB_BENCH_VERSION,
		ADHR_L2_VERSION,
		ADHR_L2_WIRE_SIZE,
		ADHR_L2_EVENT_WIRE_SIZE,
		ADHR_L2_QUEUE_CAPACITY,
		ADHR_USB_BENCH_CAP_MODE_CONTROL | ADHR_USB_BENCH_CAP_L2_INJECTION |
			ADHR_USB_BENCH_CAP_EVENT_QUEUE | ADHR_USB_BENCH_CAP_LIFECYCLE |
			ADHR_USB_BENCH_CAP_STATUS | ADHR_USB_BENCH_CAP_RX_ONLY,
		0U,
		0U,
	};
	return write_response(command, ADHR_USB_BENCH_STATUS_OK, payload, sizeof(payload), response, response_capacity);
}

static size_t status_response(uint8_t command, const adhr_usb_bench_ops_t *ops,
	uint8_t *response, size_t response_capacity)
{
	const adhr_mode_state_t *mode = adhrModeState();
	const adhr_rx_state_t *l2 = adhrL2State();
	uint8_t payload[ADHR_USB_BENCH_STATUS_PAYLOAD_SIZE] = {0};
	payload[0] = (uint8_t)mode->mode;
	payload[1] = (uint8_t)mode->last_reason;
	payload[2] = (uint8_t)mode->last_error;
	payload[3] = physical_tx_flags(ops);
	payload[4] = (adhrModeTXAllowed() || adhrL2TXAllowed()) ? 1U : 0U;
	payload[5] = l2->pending_count;
	put_u32_be(&payload[8], mode->transition_sequence);
	put_u32_be(&payload[12], l2->sequence);
	put_u32_be(&payload[16], l2->generation);
	put_u32_be(&payload[20], l2->accepted_count);
	put_u32_be(&payload[24], l2->rejected_count);
	put_u32_be(&payload[28], l2->pressure_count);
	return write_response(command, ADHR_USB_BENCH_STATUS_OK, payload, sizeof(payload), response, response_capacity);
}

size_t adhrUsbBenchHandle(const uint8_t *request, size_t request_length,
	uint8_t *response, size_t response_capacity, const adhr_usb_bench_ops_t *ops)
{
	uint8_t command = ((request != NULL) && (request_length > 2U)) ? request[2] : 0U;
	if ((response == NULL) || (response_capacity < ADHR_USB_BENCH_RESPONSE_HEADER_SIZE))
	{
		return 0U;
	}
	if ((request == NULL) || (request_length < ADHR_USB_BENCH_REQUEST_HEADER_SIZE))
	{
		return write_response(command, ADHR_USB_BENCH_STATUS_BAD_LENGTH, NULL, 0U, response, response_capacity);
	}
	if (request[0] != ADHR_USB_BENCH_MAGIC)
	{
		return write_response(command, ADHR_USB_BENCH_STATUS_BAD_MAGIC, NULL, 0U, response, response_capacity);
	}
	if (request[1] != ADHR_USB_BENCH_VERSION)
	{
		return write_response(command, ADHR_USB_BENCH_STATUS_BAD_VERSION, NULL, 0U, response, response_capacity);
	}
	if (request_length != (ADHR_USB_BENCH_REQUEST_HEADER_SIZE + request[3]))
	{
		return write_response(command, ADHR_USB_BENCH_STATUS_BAD_LENGTH, NULL, 0U, response, response_capacity);
	}

	const uint8_t *payload = &request[ADHR_USB_BENCH_REQUEST_HEADER_SIZE];
	size_t payload_length = request[3];
	switch ((adhr_usb_bench_command_t)command)
	{
		case ADHR_USB_BENCH_COMMAND_IDENTITY:
			if (payload_length != 0U)
			{
				break;
			}
			return identity_response(command, response, response_capacity);

		case ADHR_USB_BENCH_COMMAND_STATUS:
			if (payload_length != 0U)
			{
				break;
			}
			return status_response(command, ops, response, response_capacity);

		case ADHR_USB_BENCH_COMMAND_SET_MODE:
			if ((payload_length != 1U) ||
				((payload[0] != ADHR_MODE_ANALOG_FM) && (payload[0] != ADHR_MODE_ADHR)))
			{
				break;
			}
			if (!tx_guard_closed(ops))
			{
				return write_response(command, ADHR_USB_BENCH_STATUS_TX_GUARD, NULL, 0U, response, response_capacity);
			}
			if ((ops == NULL) || (ops->set_mode == NULL) ||
				!ops->set_mode((adhr_mode_t)payload[0]) || (adhrModeCurrent() != (adhr_mode_t)payload[0]))
			{
				return write_response(command, ADHR_USB_BENCH_STATUS_MODE_REJECTED, NULL, 0U, response, response_capacity);
			}
			if (!tx_guard_closed(ops))
			{
				return write_response(command, ADHR_USB_BENCH_STATUS_TX_GUARD, NULL, 0U, response, response_capacity);
			}
			{
				uint8_t current = (uint8_t)adhrModeCurrent();
				return write_response(command, ADHR_USB_BENCH_STATUS_OK, &current, 1U, response, response_capacity);
			}

		case ADHR_USB_BENCH_COMMAND_INJECT_L2:
			if (payload_length != ADHR_L2_WIRE_SIZE)
			{
				break;
			}
			if (!tx_guard_closed(ops))
			{
				return write_response(command, ADHR_USB_BENCH_STATUS_TX_GUARD, NULL, 0U, response, response_capacity);
			}
			{
				adhr_rx_result_t result = adhrL2Process(adhrModeCurrent(), payload, payload_length, observed_tick(ops));
				uint8_t result_payload[8] = {
					result.consumed ? 1U : 0U,
					result.event_queued ? 1U : 0U,
					(uint8_t)result.reason,
					0U,
					0U, 0U, 0U, 0U,
				};
				put_u32_be(&result_payload[4], result.sequence);
				return write_response(command, ADHR_USB_BENCH_STATUS_OK, result_payload, sizeof(result_payload), response, response_capacity);
			}

		case ADHR_USB_BENCH_COMMAND_POP_EVENT:
			if (payload_length != 0U)
			{
				break;
			}
			{
				adhr_rx_event_t event;
				uint8_t event_payload[ADHR_L2_EVENT_WIRE_SIZE];
				if (!adhrL2PopEvent(&event))
				{
					return write_response(command, ADHR_USB_BENCH_STATUS_NO_EVENT, NULL, 0U, response, response_capacity);
				}
				if (!adhrL2SerializeEvent(&event, event_payload, sizeof(event_payload)))
				{
					return write_response(command, ADHR_USB_BENCH_STATUS_INTERNAL, NULL, 0U, response, response_capacity);
				}
				return write_response(command, ADHR_USB_BENCH_STATUS_OK, event_payload, sizeof(event_payload), response, response_capacity);
			}

		case ADHR_USB_BENCH_COMMAND_LIFECYCLE:
			if ((payload_length != 1U) ||
				((payload[0] != ADHR_RX_REASON_TIMEOUT) && (payload[0] != ADHR_RX_REASON_RESET) &&
				 (payload[0] != ADHR_RX_REASON_DISCONNECT)))
			{
				break;
			}
			if (!tx_guard_closed(ops))
			{
				return write_response(command, ADHR_USB_BENCH_STATUS_TX_GUARD, NULL, 0U, response, response_capacity);
			}
			{
				adhr_rx_reason_t reason = (adhr_rx_reason_t)payload[0];
				adhr_rx_result_t result = adhrL2Lifecycle(reason, observed_tick(ops));
				uint8_t result_payload[4] = {
					result.consumed ? 1U : 0U,
					result.event_queued ? 1U : 0U,
					(uint8_t)result.reason,
					0U,
				};
				return write_response(command, ADHR_USB_BENCH_STATUS_OK, result_payload, sizeof(result_payload), response, response_capacity);
			}

		case ADHR_USB_BENCH_COMMAND_RESET:
			if (payload_length != 0U)
			{
				break;
			}
			if (!tx_guard_closed(ops))
			{
				return write_response(command, ADHR_USB_BENCH_STATUS_TX_GUARD, NULL, 0U, response, response_capacity);
			}
			adhrL2Initialize();
			adhrModeReset(ADHR_MODE_REASON_RESET);
			return write_response(command, ADHR_USB_BENCH_STATUS_OK, NULL, 0U, response, response_capacity);

		default:
			return write_response(command, ADHR_USB_BENCH_STATUS_BAD_COMMAND, NULL, 0U, response, response_capacity);
	}

	return write_response(command, ADHR_USB_BENCH_STATUS_BAD_PAYLOAD, NULL, 0U, response, response_capacity);
}
