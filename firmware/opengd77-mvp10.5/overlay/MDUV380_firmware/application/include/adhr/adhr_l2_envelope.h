/*
 * SPDX-License-Identifier: Apache-2.0
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 */
#ifndef ADHR_L2_ENVELOPE_H
#define ADHR_L2_ENVELOPE_H

#include "adhr/adhr_mode.h"

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#define ADHR_L2_DISCRIMINATOR 0xA7U
#define ADHR_L2_VERSION 0x01U
#define ADHR_L2_PAYLOAD_SIZE 22U
#define ADHR_L2_WIRE_SIZE 27U
#define ADHR_L2_QUEUE_CAPACITY 4U
#define ADHR_L2_EVENT_WIRE_SIZE 34U

typedef enum
{
	ADHR_RX_REASON_ACCEPTED = 0,
	ADHR_RX_REASON_WRONG_MODE = 1,
	ADHR_RX_REASON_TRUNCATED = 2,
	ADHR_RX_REASON_OVERSIZE = 3,
	ADHR_RX_REASON_WRONG_DISCRIMINATOR = 4,
	ADHR_RX_REASON_WRONG_VERSION = 5,
	ADHR_RX_REASON_WRONG_LENGTH = 6,
	ADHR_RX_REASON_RESERVED = 7,
	ADHR_RX_REASON_INTEGRITY = 8,
	ADHR_RX_REASON_TIMEOUT = 9,
	ADHR_RX_REASON_QUEUE_PRESSURE = 10,
	ADHR_RX_REASON_RESET = 11,
	ADHR_RX_REASON_DISCONNECT = 12,
	ADHR_RX_REASON_INVALID_LIFECYCLE = 255
} adhr_rx_reason_t;

typedef enum
{
	ADHR_RX_EVENT_FRAME = 1,
	ADHR_RX_EVENT_REJECTED = 2,
	ADHR_RX_EVENT_LIFECYCLE = 3
} adhr_rx_event_kind_t;

typedef struct
{
	bool consumed;
	bool event_queued;
	adhr_rx_reason_t reason;
	uint32_t sequence;
} adhr_rx_result_t;

typedef struct
{
	adhr_rx_event_kind_t kind;
	adhr_rx_reason_t reason;
	uint8_t payload_length;
	uint32_t sequence;
	uint32_t observed_tick;
	uint8_t payload[ADHR_L2_PAYLOAD_SIZE];
} adhr_rx_event_t;

typedef struct
{
	uint32_t sequence;
	uint32_t generation;
	uint32_t accepted_count;
	uint32_t rejected_count;
	uint32_t pressure_count;
	uint8_t pending_count;
} adhr_rx_state_t;

void adhrL2Initialize(void);
adhr_rx_result_t adhrL2Process(adhr_mode_t mode, const uint8_t *wire, size_t length, uint32_t observed_tick);
adhr_rx_result_t adhrL2Lifecycle(adhr_rx_reason_t reason, uint32_t observed_tick);
bool adhrL2PopEvent(adhr_rx_event_t *event);
const adhr_rx_state_t *adhrL2State(void);
bool adhrL2SerializeEvent(const adhr_rx_event_t *event, uint8_t *output, size_t output_size);
bool adhrL2TXAllowed(void);

#endif
