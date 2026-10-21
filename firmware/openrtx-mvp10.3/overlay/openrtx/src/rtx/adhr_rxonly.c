/*
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

#include "rtx/adhr_rxonly.h"

#include <stdarg.h>
#include <stdio.h>
#include <string.h>

static void emit_line(adhr_rxonly_context_t *context, const char *format, ...)
{
    va_list args;
    int length;

    va_start(args, format);
    length = vsnprintf(context->output, sizeof(context->output), format, args);
    va_end(args);

    if((length < 0) || ((size_t) length >= sizeof(context->output)))
    {
        const char error[] = "ADHR1 ERROR code=internal_overflow\n";
        memcpy(context->output, error, sizeof(error));
        length = sizeof(error) - 1;
    }

    if(context->emit != NULL)
        context->emit(context->output, (size_t) length, context->emit_user);
}

static char *next_token(char **cursor)
{
    char *start;

    while(**cursor == ' ')
        (*cursor)++;

    if(**cursor == '\0')
        return NULL;

    start = *cursor;
    while((**cursor != '\0') && (**cursor != ' '))
        (*cursor)++;

    if(**cursor == ' ')
    {
        **cursor = '\0';
        (*cursor)++;
    }

    return start;
}

static bool has_no_more_tokens(char *cursor)
{
    return next_token(&cursor) == NULL;
}

static bool valid_id(const char *value)
{
    size_t length = 0;

    while(value[length] != '\0')
    {
        char c = value[length];
        bool valid = ((c >= 'a') && (c <= 'z'))
                  || ((c >= 'A') && (c <= 'Z'))
                  || ((c >= '0') && (c <= '9'))
                  || (c == '_') || (c == '-') || (c == '.');
        if(!valid || (length >= 31))
            return false;
        length++;
    }

    return length > 0;
}

static bool parse_unsigned(const char *value, uint64_t maximum,
                           uint64_t *result)
{
    uint64_t parsed = 0;
    size_t i = 0;

    if(value[0] == '\0')
        return false;

    while(value[i] != '\0')
    {
        uint8_t digit;
        if((value[i] < '0') || (value[i] > '9'))
            return false;
        digit = (uint8_t) (value[i] - '0');
        if(parsed > ((maximum - digit) / 10))
            return false;
        parsed = (parsed * 10) + digit;
        i++;
    }

    *result = parsed;
    return true;
}

static bool parse_signed(const char *value, int32_t minimum, int32_t maximum,
                         int32_t *result)
{
    bool negative = value[0] == '-';
    uint64_t magnitude;
    uint64_t limit;
    const char *digits = negative ? value + 1 : value;

    limit = negative ? (uint64_t) (-(int64_t) minimum) : (uint64_t) maximum;
    if(!parse_unsigned(digits, limit, &magnitude))
        return false;

    if(negative)
        *result = (int32_t) (-(int64_t) magnitude);
    else
        *result = (int32_t) magnitude;

    return (*result >= minimum) && (*result <= maximum);
}

static bool normalize_hex(char *value, size_t *byte_count)
{
    size_t length = strlen(value);
    size_t i;

    if((length == 0) || ((length % 2) != 0)
       || (length > (ADHR_RXONLY_MAX_FRAME_BYTES * 2)))
        return false;

    for(i = 0; i < length; i++)
    {
        char c = value[i];
        if((c >= 'A') && (c <= 'F'))
        {
            value[i] = (char) (c + ('a' - 'A'));
            continue;
        }
        if(((c >= '0') && (c <= '9')) || ((c >= 'a') && (c <= 'f')))
            continue;
        return false;
    }

    *byte_count = length / 2;
    return true;
}

static void lowercase(char *value)
{
    while(*value != '\0')
    {
        if((*value >= 'A') && (*value <= 'Z'))
            *value = (char) (*value + ('a' - 'A'));
        value++;
    }
}

static void malformed(adhr_rxonly_context_t *context, const char *operation)
{
    emit_line(context, "ADHR1 ERROR code=malformed_command operation=%s\n",
              operation);
}

static void handle_rx(adhr_rxonly_context_t *context, char *cursor,
                      uint64_t monotonic_ms)
{
    char *rssi_text = next_token(&cursor);
    char *frame = next_token(&cursor);
    size_t frame_bytes;
    int32_t rssi;

    if((rssi_text == NULL) || (frame == NULL)
       || !has_no_more_tokens(cursor)
       || !parse_signed(rssi_text, -200, 0, &rssi)
       || !normalize_hex(frame, &frame_bytes))
    {
        malformed(context, "rx");
        return;
    }

    emit_line(context,
              "ADHR1 EVENT op=receive_delivery obs_seq=%lu "
              "monotonic_ms=%llu rssi_dbm=%ld frame_bytes=%s "
              "frame_size_bytes=%zu\n",
              (unsigned long) context->observation_sequence++,
              (unsigned long long) monotonic_ms, (long) rssi, frame,
              frame_bytes);
}

static void handle_link(adhr_rxonly_context_t *context, char *cursor,
                        uint64_t monotonic_ms)
{
    char *from = next_token(&cursor);
    char *to = next_token(&cursor);
    char *rssi_text = next_token(&cursor);
    char *loss_text = next_token(&cursor);
    char *delay_text = next_token(&cursor);
    int32_t rssi;
    uint64_t loss;
    uint64_t delay;

    if((from == NULL) || (to == NULL) || (rssi_text == NULL)
       || (loss_text == NULL) || (delay_text == NULL)
       || !has_no_more_tokens(cursor) || !valid_id(from) || !valid_id(to)
       || !parse_signed(rssi_text, -200, 0, &rssi)
       || !parse_unsigned(loss_text, 100000, &loss)
       || !parse_unsigned(delay_text, 60000, &delay))
    {
        malformed(context, "link");
        return;
    }

    emit_line(context,
              "ADHR1 EVENT op=link_measurement obs_seq=%lu "
              "observed_at_ms=%llu from_station_id=%s "
              "to_station_id=%s rssi_dbm=%ld "
              "loss_milli_pct=%llu delay_ms=%llu\n",
              (unsigned long) context->observation_sequence++,
              (unsigned long long) monotonic_ms, from, to, (long) rssi,
              (unsigned long long) loss, (unsigned long long) delay);
}

static void handle_timing(adhr_rxonly_context_t *context, char *cursor,
                          uint64_t monotonic_ms)
{
    char *authority = next_token(&cursor);
    char *epoch = next_token(&cursor);
    char *acquisition_text = next_token(&cursor);
    char *drift_text = next_token(&cursor);
    uint64_t acquisition;
    int32_t drift;

    if((authority == NULL) || (epoch == NULL) || (acquisition_text == NULL)
       || (drift_text == NULL) || !has_no_more_tokens(cursor)
       || !valid_id(authority) || !valid_id(epoch)
       || !parse_unsigned(acquisition_text, 60000, &acquisition)
       || !parse_signed(drift_text, -1000000, 1000000, &drift))
    {
        malformed(context, "timing");
        return;
    }

    emit_line(context,
              "ADHR1 EVENT op=timing_observation obs_seq=%lu "
              "monotonic_ms=%llu timing_authority_id=%s "
              "timing_epoch_id=%s acquisition_ms=%llu drift_ppm=%ld\n",
              (unsigned long) context->observation_sequence++,
              (unsigned long long) monotonic_ms, authority, epoch,
              (unsigned long long) acquisition, (long) drift);
}

static void process_line(adhr_rxonly_context_t *context,
                         uint64_t monotonic_ms)
{
    char *cursor = context->line;
    char *protocol = next_token(&cursor);
    char *operation = next_token(&cursor);

    if((protocol == NULL) || (strcmp(protocol, "ADHR1") != 0)
       || (operation == NULL))
    {
        malformed(context, "protocol");
        return;
    }

    if(strcmp(operation, "PING") == 0)
    {
        if(!has_no_more_tokens(cursor))
            malformed(context, "ping");
        else
            emit_line(context, "ADHR1 OK op=ping version=1 rx_only=1\n");
    }
    else if(strcmp(operation, "RESET") == 0)
    {
        if(!has_no_more_tokens(cursor))
        {
            malformed(context, "reset");
            return;
        }
        context->observation_sequence = 0;
        context->reset_generation++;
        emit_line(context,
                  "ADHR1 EVENT op=reset reset_generation=%lu "
                  "monotonic_ms=%llu next_obs_seq=0\n",
                  (unsigned long) context->reset_generation,
                  (unsigned long long) monotonic_ms);
    }
    else if(strcmp(operation, "RX") == 0)
    {
        handle_rx(context, cursor, monotonic_ms);
    }
    else if(strcmp(operation, "LINK") == 0)
    {
        handle_link(context, cursor, monotonic_ms);
    }
    else if(strcmp(operation, "TIMING") == 0)
    {
        handle_timing(context, cursor, monotonic_ms);
    }
    else if(strcmp(operation, "SCHEDULED_TX") == 0)
    {
        emit_line(context, "ADHR1 ERROR code=unsupported_operation "
                           "operation=scheduled_transmit\n");
    }
    else
    {
        lowercase(operation);
        if(!valid_id(operation))
            malformed(context, "operation");
        else
            emit_line(context, "ADHR1 ERROR code=unknown_operation "
                               "operation=%s\n", operation);
    }
}

void adhr_rxonly_init(adhr_rxonly_context_t *context,
                      adhr_rxonly_emit_fn emit, void *user)
{
    memset(context, 0, sizeof(*context));
    context->emit = emit;
    context->emit_user = user;
}

void adhr_rxonly_feed(adhr_rxonly_context_t *context, const uint8_t *data,
                      size_t length, uint64_t monotonic_ms)
{
    size_t i;

    for(i = 0; i < length; i++)
    {
        char c = (char) data[i];
        if(context->discarding_overflow)
        {
            if((c == '\n') || (c == '\r'))
                context->discarding_overflow = false;
            continue;
        }

        if((c == '\n') || (c == '\r'))
        {
            if(context->line_length > 0)
            {
                context->line[context->line_length] = '\0';
                process_line(context, monotonic_ms);
                context->line_length = 0;
            }
            continue;
        }

        if(context->line_length == 0)
            context->partial_started_ms = monotonic_ms;

        if(context->line_length >= (sizeof(context->line) - 1))
        {
            context->line_length = 0;
            context->discarding_overflow = true;
            emit_line(context, "ADHR1 ERROR code=line_overflow\n");
            continue;
        }

        context->line[context->line_length++] = c;
    }
}

void adhr_rxonly_poll(adhr_rxonly_context_t *context, uint64_t monotonic_ms)
{
    if(monotonic_ms < context->partial_started_ms)
        return;

    if(context->discarding_overflow)
    {
        if((monotonic_ms - context->partial_started_ms)
           >= ADHR_RXONLY_COMMAND_TIMEOUT_MS)
            context->discarding_overflow = false;
        return;
    }

    if(context->line_length == 0)
        return;

    if((monotonic_ms - context->partial_started_ms)
       < ADHR_RXONLY_COMMAND_TIMEOUT_MS)
        return;

    context->line_length = 0;
    emit_line(context, "ADHR1 ERROR code=command_timeout\n");
}
