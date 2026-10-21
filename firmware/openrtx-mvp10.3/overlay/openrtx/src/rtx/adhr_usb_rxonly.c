/*
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

#include "rtx/adhr_usb_rxonly.h"

#include "drivers/usb_vcom.h"
#include "interfaces/delays.h"
#include "rtx/adhr_rxonly.h"
#include "rtx/adhr_tx_queue.h"

#include <stdio.h>
#include <stdint.h>
#include <unistd.h>

static adhr_rxonly_context_t context;
static adhr_tx_queue_t tx_queue;
static bool tx_in_flight;

static void usb_emit(const char *line, size_t length, void *user)
{
    (void) user;
    (void) adhr_tx_queue_push(&tx_queue, line, length);
}

static void complete_tx(void)
{
    if(tx_in_flight && vcom_writeReady())
    {
        adhr_tx_queue_pop(&tx_queue);
        tx_in_flight = false;
    }
}

static void queue_backpressure(void)
{
    char message[80];
    uint32_t dropped = adhr_tx_queue_dropped(&tx_queue);
    int length;

    if((dropped == 0) || (tx_queue.count >= ADHR_TX_QUEUE_CAPACITY))
        return;

    length = snprintf(message, sizeof(message),
                      "ADHR1 ERROR code=output_backpressure dropped=%lu\n",
                      (unsigned long) dropped);
    if((length > 0) && ((size_t) length < sizeof(message))
       && adhr_tx_queue_push(&tx_queue, message, (size_t) length))
        adhr_tx_queue_clear_dropped(&tx_queue);
}

static void start_one_tx(void)
{
    const char *data;
    size_t length;

    if(tx_in_flight)
        return;

    data = adhr_tx_queue_front(&tx_queue);
    length = adhr_tx_queue_front_length(&tx_queue);
    if((data != NULL) && (vcom_tryWriteBlock(data, length) > 0))
        tx_in_flight = true;
}

void adhr_usb_rxonly_init(void)
{
    adhr_rxonly_init(&context, usb_emit, NULL);
    adhr_tx_queue_init(&tx_queue);
    tx_in_flight = false;
}

void adhr_usb_rxonly_poll(void)
{
    uint8_t buffer[32];
    long long tick = getTick();
    uint64_t now = tick < 0 ? 0 : (uint64_t) tick;
    unsigned int reads;

    complete_tx();

    for(reads = 0; reads < 8; reads++)
    {
        ssize_t count = vcom_readBlock(buffer, sizeof(buffer));
        if(count <= 0)
            break;
        adhr_rxonly_feed(&context, buffer, (size_t) count, now);
    }

    adhr_rxonly_poll(&context, now);
    queue_backpressure();
    start_one_tx();
}
