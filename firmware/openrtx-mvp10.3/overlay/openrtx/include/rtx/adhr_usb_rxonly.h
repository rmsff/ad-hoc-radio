/*
 * SPDX-FileCopyrightText: Copyright 2026 Ad Hoc Radio Contributors
 *
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

#ifndef ADHR_USB_RXONLY_H
#define ADHR_USB_RXONLY_H

#ifdef __cplusplus
extern "C" {
#endif

void adhr_usb_rxonly_init(void);
void adhr_usb_rxonly_poll(void);

#ifdef __cplusplus
}
#endif

#endif /* ADHR_USB_RXONLY_H */
