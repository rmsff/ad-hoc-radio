#!/bin/sh
# Stock firmware restore for a TYT MD-UV380 10W station, run on a Linux USB host (NanoPi in the original run).
# Proven path: 2026-07-09 MD-UV380 5W D219.74 restore via radio_tool (UV3X0 wrap).
# Usage: sh restore.sh check|spi-snapshot <tag>|flash|flash-gd77|settings-read|settings-write
set -eu

ROOT=/root/adhr-radio/md-uv380-10w-restore
TOOLS=$ROOT/tools
RAW=$ROOT/fw/MD-UV380-10W-AES255_REC-D219.74-internal-0x0800c000-decrypted.bin
WRAP=$ROOT/fw/MD-UV380-10W-AES255_REC-D219.74-internal-uv3x0-wrap.bin
RAW_SHA=bcc1821207cb09c4592526dbed2749d4a017c0932800e96b2bcab1b3ed8eb925
WRAP_SHA=551d1a037f321b9b1a469a30b9df323c7efdd569d1dd8138ea19702a7dbc0e93
BASELINE=$ROOT/baseline
PYUSB=$TOOLS/pyusb/pyusb-1.3.1
STAMP=$(date +%Y%m%d-%H%M%S)
LOG=$ROOT/logs/$STAMP-${1:-none}.log

mkdir -p "$ROOT/logs" "$ROOT/snapshots"

require_sha() {
    actual=$(sha256sum "$1" | cut -d' ' -f1)
    if [ "$actual" != "$2" ]; then
        echo "SHA MISMATCH: $1 = $actual, expected $2" >&2
        exit 2
    fi
    echo "sha ok: $(basename "$1")"
}

require_single_tyt_device() {
    count=$(lsusb | grep -c '0483:df11' || true)
    if [ "$count" != "1" ]; then
        echo "expected exactly one 0483:df11 device, found $count" >&2
        exit 3
    fi
}

cmd_check() {
    echo "--- usb"; lsusb | grep -v -i 'hub' || true
    require_single_tyt_device
    echo "--- dfu-util -l"; dfu-util -l 2>&1 | grep -E 'Found|alt=' || true
    echo "--- radio_tool --list"; "$TOOLS/radio_tool" --list 2>&1 || true
    echo "--- radio_tool --info"; "$TOOLS/radio_tool" -d 0 --info 2>&1 || true
    echo "--- files"
    require_sha "$RAW" "$RAW_SHA"
    require_sha "$WRAP" "$WRAP_SHA"
    "$TOOLS/radio_tool" --fw-info -i "$WRAP"
}

cmd_spi_snapshot() {
    tag=${1:?tag required}
    out=$ROOT/snapshots/spi-$tag-$STAMP.bin
    require_single_tyt_device
    PYTHONPATH=$PYUSB python3 "$TOOLS/md380_read_spi.py" "$out"
    python3 "$ROOT/spi_compare.py" "$BASELINE/spi-flash-vendor-a.bin" "$out"
}

cmd_flash() {
    require_single_tyt_device
    require_sha "$WRAP" "$WRAP_SHA"
    "$TOOLS/radio_tool" --list
    "$TOOLS/radio_tool" -d 0 -f -i "$WRAP"
    sync
    echo "flash finished: power-cycle the radio in normal mode and check Menu > Utilities > Radio Info > Versions"
}

cmd_flash_gd77() {
    require_single_tyt_device
    require_sha "$RAW" "$RAW_SHA"
    PYTHONPATH=$PYUSB python3 "$TOOLS/opengd77_stm32_firmware_loader.py" -m MD-UV380 -f "$RAW"
    sync
}

cmd_settings_read() {
    require_single_tyt_device
    dir=$ROOT/snapshots/codeplug-$STAMP
    mkdir -p "$dir"
    set +e
    (cd "$dir" && timeout 120 "$TOOLS/dmrconfig" -r -t > stdout.log 2> stderr.log)
    rc=$?
    set -e
    echo "$rc" > "$dir/rc.txt"
    echo "dmrconfig rc=$rc"; tail -3 "$dir/stdout.log"; tail -3 "$dir/stderr.log"
    [ "$rc" = "0" ] || exit "$rc"
    sha256sum "$dir/device.img" "$dir/device.conf" "$BASELINE/device.img" "$BASELINE/device.conf"
    if cmp -s "$dir/device.conf" "$BASELINE/device.conf"; then
        echo "device.conf: IDENTICAL to 2026-07-14 backup"
    else
        echo "device.conf: DIFFERS from 2026-07-14 backup"
        diff "$BASELINE/device.conf" "$dir/device.conf" | head -40 || true
    fi
    cmp -l "$BASELINE/device.img" "$dir/device.img" | wc -l | sed 's/^/device.img differing bytes: /'
}

cmd_settings_write() {
    require_single_tyt_device
    require_sha "$BASELINE/device.img" ea72cdb13e9866a09c84e260916b82cc2b58c15f23f39090d90de8ee613120c3
    timeout 180 "$TOOLS/dmrconfig" -w -t "$BASELINE/device.img"
    echo "written; now run: sh restore.sh settings-read"
}

main() {
    case "${1:-}" in
        check) cmd_check ;;
        spi-snapshot) shift; cmd_spi_snapshot "$@" ;;
        flash) cmd_flash ;;
        flash-gd77) cmd_flash_gd77 ;;
        settings-read) cmd_settings_read ;;
        settings-write) cmd_settings_write ;;
        *) echo "usage: sh restore.sh check|spi-snapshot <tag>|flash|flash-gd77|settings-read|settings-write" >&2; exit 1 ;;
    esac
}

# tee hides the step's exit code, so the step records it and the script re-reads it.
set +e
{
    (set -e; main "$@")
    echo "exit_code=$?"
} 2>&1 | tee "$LOG"
rc=$(sed -n 's/^exit_code=//p' "$LOG" | tail -1)
echo "log: $LOG"
exit "${rc:-1}"
