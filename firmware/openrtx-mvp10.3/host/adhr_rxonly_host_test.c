/*
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

#include "rtx/adhr_rxonly.h"
#include "rtx/adhr_tx_queue.h"

#include <assert.h>
#include <stdio.h>
#include <string.h>

#define MAX_CAPTURED_LINES 32

static char captured[MAX_CAPTURED_LINES][ADHR_RXONLY_MAX_OUTPUT];
static size_t captured_count;

static void capture_line(const char *line, size_t length, void *user)
{
    (void) user;
    assert(captured_count < MAX_CAPTURED_LINES);
    assert(length < ADHR_RXONLY_MAX_OUTPUT);
    memcpy(captured[captured_count], line, length);
    captured[captured_count][length] = '\0';
    captured_count++;
}

static void clear_capture(void)
{
    memset(captured, 0, sizeof(captured));
    captured_count = 0;
}

static void feed_line(adhr_rxonly_context_t *context, const char *line,
                      uint64_t monotonic_ms)
{
    adhr_rxonly_feed(context, (const uint8_t *) line, strlen(line),
                     monotonic_ms);
}

static void expect_line(size_t index, const char *expected)
{
    assert(index < captured_count);
    if(strcmp(captured[index], expected) != 0)
    {
        fprintf(stderr, "line[%zu]\nwant: %s\ngot:  %s\n", index,
                expected, captured[index]);
        assert(0);
    }
}

static void test_success_and_reset(void)
{
    adhr_rxonly_context_t context;
    adhr_rxonly_init(&context, capture_line, NULL);
    clear_capture();

    feed_line(&context, "ADHR1 PING\n", 10);
    feed_line(&context, "ADHR1 RX -87 Aa10ff\n", 11);
    feed_line(&context, "ADHR1 LINK source relay -87 125 7\n", 12);
    feed_line(&context, "ADHR1 TIMING source epoch_1 24 -350\n", 13);
    feed_line(&context, "ADHR1 RESET\n", 14);
    feed_line(&context, "ADHR1 RX -90 01\n", 15);

    assert(captured_count == 6);
    expect_line(0, "ADHR1 OK op=ping version=1 rx_only=1\n");
    expect_line(1, "ADHR1 EVENT op=receive_delivery obs_seq=0 "
                   "monotonic_ms=11 rssi_dbm=-87 frame_bytes=aa10ff "
                   "frame_size_bytes=3\n");
    expect_line(2, "ADHR1 EVENT op=link_measurement obs_seq=1 "
                   "observed_at_ms=12 from_station_id=source "
                   "to_station_id=relay rssi_dbm=-87 "
                   "loss_milli_pct=125 delay_ms=7\n");
    expect_line(3, "ADHR1 EVENT op=timing_observation obs_seq=2 "
                   "monotonic_ms=13 timing_authority_id=source "
                   "timing_epoch_id=epoch_1 acquisition_ms=24 "
                   "drift_ppm=-350\n");
    expect_line(4, "ADHR1 EVENT op=reset reset_generation=1 "
                   "monotonic_ms=14 next_obs_seq=0\n");
    expect_line(5, "ADHR1 EVENT op=receive_delivery obs_seq=0 "
                   "monotonic_ms=15 rssi_dbm=-90 frame_bytes=01 "
                   "frame_size_bytes=1\n");
}

static void test_errors_and_tx_rejection(void)
{
    adhr_rxonly_context_t context;
    adhr_rxonly_init(&context, capture_line, NULL);
    clear_capture();

    feed_line(&context, "ADHR1 RX -87 0xz1\n", 20);
    feed_line(&context, "ADHR1 LINK a b -201 0 1\n", 21);
    feed_line(&context, "ADHR1 TIMING a b 18446744073709551616 0\n", 22);
    feed_line(&context, "ADHR1 SCHEDULED_TX ignored\n", 23);
    feed_line(&context, "ADHR1 ROUTE a b\n", 24);
    feed_line(&context, "BAD PING\n", 25);

    assert(captured_count == 6);
    expect_line(0, "ADHR1 ERROR code=malformed_command operation=rx\n");
    expect_line(1, "ADHR1 ERROR code=malformed_command operation=link\n");
    expect_line(2, "ADHR1 ERROR code=malformed_command operation=timing\n");
    expect_line(3, "ADHR1 ERROR code=unsupported_operation "
                   "operation=scheduled_transmit\n");
    expect_line(4, "ADHR1 ERROR code=unknown_operation operation=route\n");
    expect_line(5, "ADHR1 ERROR code=malformed_command operation=protocol\n");
}

static void fill_hex(char *output, size_t bytes)
{
    size_t i;

    for(i = 0; i < bytes; i++)
    {
        output[i * 2] = 'a';
        output[(i * 2) + 1] = '5';
    }
    output[bytes * 2] = '\0';
}

static void test_frame_boundaries(void)
{
    adhr_rxonly_context_t context;
    char command[(ADHR_RXONLY_MAX_FRAME_BYTES * 2) + 24];
    char frame[(ADHR_RXONLY_MAX_FRAME_BYTES * 2) + 3];
    char expected[ADHR_RXONLY_MAX_OUTPUT];

    adhr_rxonly_init(&context, capture_line, NULL);
    clear_capture();

    fill_hex(frame, ADHR_RXONLY_MAX_FRAME_BYTES);
    snprintf(command, sizeof(command), "ADHR1 RX -100 %s\n", frame);
    snprintf(expected, sizeof(expected),
             "ADHR1 EVENT op=receive_delivery obs_seq=0 monotonic_ms=30 "
             "rssi_dbm=-100 frame_bytes=%s frame_size_bytes=%d\n",
             frame, ADHR_RXONLY_MAX_FRAME_BYTES);
    feed_line(&context, command, 30);

    fill_hex(frame, ADHR_RXONLY_MAX_FRAME_BYTES + 1);
    snprintf(command, sizeof(command), "ADHR1 RX -100 %s\n", frame);
    feed_line(&context, command, 31);

    assert(captured_count == 2);
    expect_line(0, expected);
    expect_line(1, "ADHR1 ERROR code=malformed_command operation=rx\n");
}

static void test_numeric_id_and_token_boundaries(void)
{
    adhr_rxonly_context_t context;
    adhr_rxonly_init(&context, capture_line, NULL);
    clear_capture();

    feed_line(&context,
              "ADHR1 LINK abcdefghijklmnopqrstuvwxyz12345 dst -200 "
              "100000 60000\n",
              40);
    feed_line(&context,
              "ADHR1 TIMING source epoch 60000 -1000000\n",
              41);
    feed_line(&context,
              "ADHR1 TIMING source epoch 60000 1000000\n",
              42);
    feed_line(&context,
              "ADHR1 LINK abcdefghijklmnopqrstuvwxyz123456 dst 0 0 0\n",
              43);
    feed_line(&context, "ADHR1 LINK a b 0 100001 0\n", 44);
    feed_line(&context, "ADHR1 LINK a b 0 0 60001\n", 45);
    feed_line(&context, "ADHR1 TIMING a b 0 -1000001\n", 46);
    feed_line(&context, "ADHR1 PING extra\n", 47);
    feed_line(&context, "ADHR1 RX -1 aa extra\n", 48);

    assert(captured_count == 9);
    expect_line(0, "ADHR1 EVENT op=link_measurement obs_seq=0 "
                   "observed_at_ms=40 "
                   "from_station_id=abcdefghijklmnopqrstuvwxyz12345 "
                   "to_station_id=dst "
                   "rssi_dbm=-200 loss_milli_pct=100000 "
                   "delay_ms=60000\n");
    expect_line(1, "ADHR1 EVENT op=timing_observation obs_seq=1 "
                   "monotonic_ms=41 timing_authority_id=source "
                   "timing_epoch_id=epoch acquisition_ms=60000 "
                   "drift_ppm=-1000000\n");
    expect_line(2, "ADHR1 EVENT op=timing_observation obs_seq=2 "
                   "monotonic_ms=42 timing_authority_id=source "
                   "timing_epoch_id=epoch acquisition_ms=60000 "
                   "drift_ppm=1000000\n");
    expect_line(3, "ADHR1 ERROR code=malformed_command operation=link\n");
    expect_line(4, "ADHR1 ERROR code=malformed_command operation=link\n");
    expect_line(5, "ADHR1 ERROR code=malformed_command operation=link\n");
    expect_line(6,
                "ADHR1 ERROR code=malformed_command operation=timing\n");
    expect_line(7, "ADHR1 ERROR code=malformed_command operation=ping\n");
    expect_line(8, "ADHR1 ERROR code=malformed_command operation=rx\n");
}

static void test_timeout_overflow_and_recovery(void)
{
    adhr_rxonly_context_t context;
    char oversized[ADHR_RXONLY_MAX_LINE + 8];
    adhr_rxonly_init(&context, capture_line, NULL);
    clear_capture();

    adhr_rxonly_feed(&context, (const uint8_t *) "ADHR1 RX -1 aa",
                     sizeof("ADHR1 RX -1 aa") - 1, 100);
    adhr_rxonly_poll(&context, 100 + ADHR_RXONLY_COMMAND_TIMEOUT_MS - 1);
    assert(captured_count == 0);
    adhr_rxonly_poll(&context, 100 + ADHR_RXONLY_COMMAND_TIMEOUT_MS);
    assert(captured_count == 1);
    expect_line(0, "ADHR1 ERROR code=command_timeout\n");
    adhr_rxonly_poll(&context, 100 + ADHR_RXONLY_COMMAND_TIMEOUT_MS + 1);
    assert(captured_count == 1);

    memset(oversized, 'A', sizeof(oversized));
    oversized[sizeof(oversized) - 1] = '\n';
    adhr_rxonly_feed(&context, (const uint8_t *) oversized,
                     sizeof(oversized), 2000);
    assert(captured_count == 2);
    expect_line(1, "ADHR1 ERROR code=line_overflow\n");

    feed_line(&context, "ADHR1 PING\n", 2001);
    assert(captured_count == 3);
    expect_line(2, "ADHR1 OK op=ping version=1 rx_only=1\n");

    memset(oversized, 'B', sizeof(oversized));
    adhr_rxonly_feed(&context, (const uint8_t *) oversized,
                     sizeof(oversized), 3000);
    assert(captured_count == 4);
    expect_line(3, "ADHR1 ERROR code=line_overflow\n");
    adhr_rxonly_poll(&context,
                     3000 + ADHR_RXONLY_COMMAND_TIMEOUT_MS);
    feed_line(&context, "ADHR1 PING\n", 4001);
    assert(captured_count == 5);
    expect_line(4, "ADHR1 OK op=ping version=1 rx_only=1\n");
}

static void test_tx_queue_backpressure(void)
{
    adhr_tx_queue_t queue;
    const char *messages[] = {"one", "two", "three", "four"};
    size_t i;

    adhr_tx_queue_init(&queue);
    for(i = 0; i < ADHR_TX_QUEUE_CAPACITY; i++)
        assert(adhr_tx_queue_push(&queue, messages[i], strlen(messages[i])));

    assert(!adhr_tx_queue_push(&queue, "dropped", 7));
    assert(adhr_tx_queue_dropped(&queue) == 1);
    assert(adhr_tx_queue_front_length(&queue) == 3);
    assert(memcmp(adhr_tx_queue_front(&queue), "one", 3) == 0);

    adhr_tx_queue_pop(&queue);
    assert(adhr_tx_queue_push(&queue, "after", 5));
    assert(memcmp(adhr_tx_queue_front(&queue), "two", 3) == 0);
    adhr_tx_queue_clear_dropped(&queue);
    assert(adhr_tx_queue_dropped(&queue) == 0);
}

int main(void)
{
    test_success_and_reset();
    test_errors_and_tx_rejection();
    test_frame_boundaries();
    test_numeric_id_and_token_boundaries();
    test_timeout_overflow_and_recovery();
    test_tx_queue_backpressure();
    puts("adhr_rxonly_host_test: PASS");
    return 0;
}
