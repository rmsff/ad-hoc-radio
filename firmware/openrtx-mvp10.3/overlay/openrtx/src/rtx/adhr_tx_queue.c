/*
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

#include "rtx/adhr_tx_queue.h"

#include <string.h>

void adhr_tx_queue_init(adhr_tx_queue_t *queue)
{
    memset(queue, 0, sizeof(*queue));
}

bool adhr_tx_queue_push(adhr_tx_queue_t *queue, const char *data,
                        size_t length)
{
    uint8_t slot;

    if((length > ADHR_RXONLY_MAX_OUTPUT)
       || (queue->count >= ADHR_TX_QUEUE_CAPACITY))
    {
        queue->dropped++;
        return false;
    }

    slot = (uint8_t) ((queue->head + queue->count)
                      % ADHR_TX_QUEUE_CAPACITY);
    memcpy(queue->data[slot], data, length);
    queue->lengths[slot] = (uint16_t) length;
    queue->count++;
    return true;
}

const char *adhr_tx_queue_front(const adhr_tx_queue_t *queue)
{
    if(queue->count == 0)
        return NULL;
    return queue->data[queue->head];
}

size_t adhr_tx_queue_front_length(const adhr_tx_queue_t *queue)
{
    if(queue->count == 0)
        return 0;
    return queue->lengths[queue->head];
}

void adhr_tx_queue_pop(adhr_tx_queue_t *queue)
{
    if(queue->count == 0)
        return;
    queue->head = (uint8_t) ((queue->head + 1)
                             % ADHR_TX_QUEUE_CAPACITY);
    queue->count--;
}

uint32_t adhr_tx_queue_dropped(const adhr_tx_queue_t *queue)
{
    return queue->dropped;
}

void adhr_tx_queue_clear_dropped(adhr_tx_queue_t *queue)
{
    queue->dropped = 0;
}
