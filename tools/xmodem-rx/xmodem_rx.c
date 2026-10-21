/*
 * Minimal XMODEM/CRC receiver for OpenRTX external-flash backup over USB CDC.
 *
 * Usage:
 *   xmodem_rx /dev/ttyACM0 /tmp/openrtx-eflash.bin
 *
 * It only receives data from the radio after the radio UI starts Backup.
 * It never writes to the radio flash.
 */

#include <errno.h>
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/select.h>
#include <termios.h>
#include <time.h>
#include <unistd.h>

#define SOH 0x01
#define STX 0x02
#define EOT 0x04
#define ACK 0x06
#define NAK 0x15
#define CAN 0x18
#define CRC_REQ 0x43

static uint16_t crc_ccitt(const uint8_t *data, size_t len)
{
    uint16_t crc = 0;

    for(size_t i = 0; i < len; i++)
    {
        crc ^= (uint16_t) data[i] << 8;
        for(int bit = 0; bit < 8; bit++)
        {
            if((crc & 0x8000) != 0)
            {
                crc = (uint16_t) ((crc << 1) ^ 0x1021);
            }
            else
            {
                crc = (uint16_t) (crc << 1);
            }
        }
    }

    return crc;
}

static int wait_byte(int fd, uint8_t *byte, int timeout_ms)
{
    fd_set rfds;
    FD_ZERO(&rfds);
    FD_SET(fd, &rfds);

    struct timeval tv;
    tv.tv_sec = timeout_ms / 1000;
    tv.tv_usec = (timeout_ms % 1000) * 1000;

    int ready = select(fd + 1, &rfds, NULL, NULL, &tv);
    if(ready < 0) return -1;
    if(ready == 0) return 0;

    ssize_t n = read(fd, byte, 1);
    if(n == 1) return 1;
    if(n == 0) return 0;
    return -1;
}

static int read_exact(int fd, uint8_t *buf, size_t len, int timeout_ms)
{
    size_t pos = 0;

    while(pos < len)
    {
        uint8_t byte = 0;
        int rc = wait_byte(fd, &byte, timeout_ms);
        if(rc <= 0) return rc;
        buf[pos++] = byte;
    }

    return 1;
}

static int write_byte(int fd, uint8_t byte)
{
    return write(fd, &byte, 1) == 1 ? 0 : -1;
}

static int set_raw_serial(int fd, struct termios *old_tio)
{
    if(tcgetattr(fd, old_tio) != 0) return -1;

    struct termios tio = *old_tio;
    tio.c_iflag &= (tcflag_t) ~(IGNBRK | BRKINT | PARMRK | ISTRIP | INLCR |
                                IGNCR | ICRNL | IXON);
    tio.c_oflag &= (tcflag_t) ~OPOST;
    tio.c_lflag &= (tcflag_t) ~(ECHO | ECHONL | ICANON | ISIG | IEXTEN);
    tio.c_cflag &= (tcflag_t) ~(CSIZE | PARENB);
    tio.c_cflag |= CS8;
    cfsetispeed(&tio, B115200);
    cfsetospeed(&tio, B115200);
    tio.c_cflag |= (CLOCAL | CREAD);
#ifdef CRTSCTS
    tio.c_cflag &= ~CRTSCTS;
#endif

    if(tcsetattr(fd, TCSANOW, &tio) != 0) return -1;
    tcflush(fd, TCIOFLUSH);
    return 0;
}

int main(int argc, char **argv)
{
    if(argc != 3)
    {
        fprintf(stderr, "usage: %s <serial-device> <output-file>\n", argv[0]);
        return 2;
    }

    const char *serial_path = argv[1];
    const char *output_path = argv[2];

    int serial_fd = open(serial_path, O_RDWR | O_NOCTTY | O_SYNC);
    if(serial_fd < 0)
    {
        fprintf(stderr, "open serial failed: %s: %s\n", serial_path, strerror(errno));
        return 1;
    }

    struct termios old_tio;
    if(set_raw_serial(serial_fd, &old_tio) != 0)
    {
        fprintf(stderr, "serial raw mode failed: %s\n", strerror(errno));
        close(serial_fd);
        return 1;
    }

    FILE *out = fopen(output_path, "wb");
    if(out == NULL)
    {
        fprintf(stderr, "open output failed: %s: %s\n", output_path, strerror(errno));
        tcsetattr(serial_fd, TCSANOW, &old_tio);
        close(serial_fd);
        return 1;
    }

    uint8_t expected_block = 1;
    uint8_t previous_block = 0;
    uint8_t data[1024];
    uint8_t header[2];
    uint8_t crc_bytes[2];
    size_t written = 0;
    time_t start = time(NULL);
    time_t last_progress = start;
    int started = 0;
    int idle_rounds = 0;

    fprintf(stderr, "Waiting for XMODEM sender on %s. Start Backup on radio UI now.\n",
            serial_path);

    while(1)
    {
        if(!started)
        {
            if(write_byte(serial_fd, CRC_REQ) != 0)
            {
                fprintf(stderr, "failed to send CRC request: %s\n", strerror(errno));
                break;
            }
        }

        uint8_t marker = 0;
        int rc = wait_byte(serial_fd, &marker, started ? 30000 : 1000);
        if(rc < 0)
        {
            fprintf(stderr, "serial read failed: %s\n", strerror(errno));
            break;
        }
        if(rc == 0)
        {
            idle_rounds++;
            if(!started && idle_rounds > 180)
            {
                fprintf(stderr, "timeout waiting for radio backup start\n");
                break;
            }
            if(started)
            {
                write_byte(serial_fd, NAK);
            }
            continue;
        }

        idle_rounds = 0;

        if(marker == EOT)
        {
            write_byte(serial_fd, ACK);
            fprintf(stderr, "Done. received=%zu bytes elapsed=%lds\n",
                    written, (long) (time(NULL) - start));
            fclose(out);
            tcsetattr(serial_fd, TCSANOW, &old_tio);
            close(serial_fd);
            return 0;
        }

        if(marker == CAN)
        {
            fprintf(stderr, "transfer canceled by sender\n");
            break;
        }

        size_t block_size = 0;
        if(marker == SOH)
        {
            block_size = 128;
        }
        else if(marker == STX)
        {
            block_size = 1024;
        }
        else
        {
            if(started) write_byte(serial_fd, NAK);
            continue;
        }

        started = 1;

        if(read_exact(serial_fd, header, sizeof(header), 30000) <= 0 ||
           read_exact(serial_fd, data, block_size, 30000) <= 0 ||
           read_exact(serial_fd, crc_bytes, sizeof(crc_bytes), 30000) <= 0)
        {
            fprintf(stderr, "timeout while receiving block %u\n", expected_block);
            write_byte(serial_fd, NAK);
            continue;
        }

        uint8_t block = header[0];
        uint8_t inverse = header[1];
        uint16_t sent_crc = (uint16_t) crc_bytes[0] << 8 | crc_bytes[1];
        uint16_t calc_crc = crc_ccitt(data, block_size);

        if((uint8_t) (block ^ inverse) != 0xFF || sent_crc != calc_crc)
        {
            fprintf(stderr, "bad block seq=%u expected=%u crc=%04x/%04x\n",
                    block, expected_block, sent_crc, calc_crc);
            write_byte(serial_fd, NAK);
            continue;
        }

        if(block == expected_block)
        {
            if(fwrite(data, 1, block_size, out) != block_size)
            {
                fprintf(stderr, "write output failed: %s\n", strerror(errno));
                break;
            }
            written += block_size;
            previous_block = block;
            expected_block++;

            time_t now = time(NULL);
            if(written % (1024 * 1024) == 0 || now - last_progress >= 5)
            {
                fprintf(stderr, "received=%zu bytes\n", written);
                last_progress = now;
            }
        }
        else if(block != previous_block)
        {
            fprintf(stderr, "unexpected block seq=%u expected=%u previous=%u\n",
                    block, expected_block, previous_block);
            write_byte(serial_fd, NAK);
            continue;
        }

        write_byte(serial_fd, ACK);
    }

    fclose(out);
    tcsetattr(serial_fd, TCSANOW, &old_tio);
    close(serial_fd);
    return 1;
}
