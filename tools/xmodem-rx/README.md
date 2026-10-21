# XMODEM RX helper

Minimal host-side XMODEM/CRC receiver for OpenRTX external-flash backup over
USB CDC.

Build:

```sh
cc -std=c11 -Wall -Wextra -O2 tools/xmodem-rx/xmodem_rx.c -o xmodem_rx
```

Use on the USB host connected to the radio:

```sh
./xmodem_rx /dev/ttyACM0 /tmp/openrtx-eflash-backup.bin
```

On the OpenRTX MD-UV380 build used in these experiments, `Backup & Restore` was
not exposed from the main menu. Use this helper only after a later firmware patch exposes a safe
backup trigger.

The helper only receives an XMODEM stream and writes it to a local file. It does
not write to radio flash. Do not use it with the OpenRTX `Restore` menu item.
