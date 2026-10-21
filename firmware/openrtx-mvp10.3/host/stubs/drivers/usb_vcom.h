#ifndef ADHR_HOST_STUB_USB_VCOM_H
#define ADHR_HOST_STUB_USB_VCOM_H

#include <stdbool.h>
#include <stddef.h>
#include <unistd.h>

bool vcom_writeReady(void);
ssize_t vcom_tryWriteBlock(const void *buf, size_t len);
ssize_t vcom_readBlock(void *buf, size_t len);

#endif
