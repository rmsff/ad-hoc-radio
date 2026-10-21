/*
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

#include "rtx/adhr_usb_rxonly.h"

#include <assert.h>
#include <stdbool.h>
#include <stdio.h>
#include <string.h>

#define OUTPUT_CAPACITY 8
#define OUTPUT_BYTES 512

static const char input_stream[] =
    "ADHR1 PING\nADHR1 PING\nADHR1 PING\n"
    "ADHR1 PING\nADHR1 PING\nADHR1 PING\n";
static size_t input_offset;
static bool endpoint_ready;
static unsigned int try_calls_this_poll;
static char outputs[OUTPUT_CAPACITY][OUTPUT_BYTES];
static size_t output_count;

long long getTick(void)
{
    return 42;
}

ssize_t vcom_readBlock(void *buffer, size_t length)
{
    size_t remaining = strlen(input_stream) - input_offset;
    size_t count = remaining < length ? remaining : length;

    if(count == 0)
        return 0;
    memcpy(buffer, input_stream + input_offset, count);
    input_offset += count;
    return (ssize_t) count;
}

bool vcom_writeReady(void)
{
    return endpoint_ready;
}

ssize_t vcom_tryWriteBlock(const void *buffer, size_t length)
{
    try_calls_this_poll++;
    if((length == 0) || !endpoint_ready)
        return 0;

    assert(output_count < OUTPUT_CAPACITY);
    assert(length < OUTPUT_BYTES);
    memcpy(outputs[output_count], buffer, length);
    outputs[output_count][length] = '\0';
    output_count++;
    endpoint_ready = false;
    return (ssize_t) length;
}

static void poll_once(void)
{
    try_calls_this_poll = 0;
    adhr_usb_rxonly_poll();
    assert(try_calls_this_poll <= 1);
}

int main(void)
{
    size_t i;

    endpoint_ready = false;
    adhr_usb_rxonly_init();

    poll_once();
    assert(input_offset == strlen(input_stream));
    assert(output_count == 0);

    poll_once();
    assert(output_count == 0);

    endpoint_ready = true;
    for(i = 0; (i < 10) && (output_count < 5); i++)
    {
        poll_once();
        endpoint_ready = true;
    }
    assert(output_count == 5);

    for(i = 0; i < 4; i++)
    {
        assert(strcmp(outputs[i],
                      "ADHR1 OK op=ping version=1 rx_only=1\n") == 0);
    }
    assert(strcmp(outputs[4],
                  "ADHR1 ERROR code=output_backpressure dropped=2\n") == 0);

    puts("adhr_usb_rxonly_host_test: PASS");
    return 0;
}
