/*
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

#ifndef ADHR_TX_QUEUE_H
#define ADHR_TX_QUEUE_H

#include "rtx/adhr_rxonly.h"

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define ADHR_TX_QUEUE_CAPACITY 4

typedef struct
{
    char data[ADHR_TX_QUEUE_CAPACITY][ADHR_RXONLY_MAX_OUTPUT];
    uint16_t lengths[ADHR_TX_QUEUE_CAPACITY];
    uint32_t dropped;
    uint8_t head;
    uint8_t count;
} adhr_tx_queue_t;

void adhr_tx_queue_init(adhr_tx_queue_t *queue);

bool adhr_tx_queue_push(adhr_tx_queue_t *queue, const char *data,
                        size_t length);

const char *adhr_tx_queue_front(const adhr_tx_queue_t *queue);

size_t adhr_tx_queue_front_length(const adhr_tx_queue_t *queue);

void adhr_tx_queue_pop(adhr_tx_queue_t *queue);

uint32_t adhr_tx_queue_dropped(const adhr_tx_queue_t *queue);

void adhr_tx_queue_clear_dropped(adhr_tx_queue_t *queue);

#ifdef __cplusplus
}
#endif

#endif /* ADHR_TX_QUEUE_H */
