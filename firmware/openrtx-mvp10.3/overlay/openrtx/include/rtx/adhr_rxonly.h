/*
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

#ifndef ADHR_RXONLY_H
#define ADHR_RXONLY_H

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define ADHR_RXONLY_MAX_LINE 384
#define ADHR_RXONLY_MAX_OUTPUT 512
#define ADHR_RXONLY_MAX_FRAME_BYTES 128
#define ADHR_RXONLY_COMMAND_TIMEOUT_MS 1000

typedef void (*adhr_rxonly_emit_fn)(const char *line, size_t length,
                                    void *user);

typedef struct
{
    char line[ADHR_RXONLY_MAX_LINE];
    char output[ADHR_RXONLY_MAX_OUTPUT];
    size_t line_length;
    uint64_t partial_started_ms;
    uint32_t observation_sequence;
    uint32_t reset_generation;
    bool discarding_overflow;
    adhr_rxonly_emit_fn emit;
    void *emit_user;
} adhr_rxonly_context_t;

void adhr_rxonly_init(adhr_rxonly_context_t *context,
                      adhr_rxonly_emit_fn emit, void *user);

void adhr_rxonly_feed(adhr_rxonly_context_t *context, const uint8_t *data,
                      size_t length, uint64_t monotonic_ms);

void adhr_rxonly_poll(adhr_rxonly_context_t *context,
                      uint64_t monotonic_ms);

#ifdef __cplusplus
}
#endif

#endif /* ADHR_RXONLY_H */
