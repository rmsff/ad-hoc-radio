---
title: OpenRTX MD-UV380 Bench Entry Plan
status: draft
type: firmware-bench-entry-plan
updated: 2026-07-08
---

# OpenRTX MD-UV380 Bench Entry Plan

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

This document converts the physical TYT MD-UV380 intake into the first OpenRTX
receive-only firmware step for ADHR. It records the firmware write once
performed; it does not authorize RF transmission.

## Verdict

```text
station_intake = pass_readonly
stock_restore_family = MD-UV380-AES255(REC)-D219.74.bin
stock_usb_live_control = not_observed
openrtx_source_pin = v0.4.4 / 801fc8dd313eb3ac6f83fd4d3b63ed895effdd56
openrtx_target = mduv3x0
openrtx_baseline_build = pass
receive_only_patch_build = pass
first_boot_flash_plan_drafted = true
openrtx_receive_only_write = pass
first_boot_confirmed = true
analog_nfm_rx_observed = true
usb_telemetry_path = pass
setfm_setnfm_patch_build = pass
setfm_setnfm_write = pass
snap_patch_build = pass
snap_write = pass
next_firmware_step = normal_boot_and_verify_setnfm_snap_then_capture_receive_events
firmware_flash_ready = consumed_by_2026_07_08_write
hardware_claim_allowed = false
field_tx_ready = false
```

The station-independent OpenRTX patch spike now compiles and proves its size
guard. The first write of the receive-only wrapped artifact completed on
2026-07-08. First boot is confirmed by normal-mode USB CDC and user-visible
OpenRTX UI.

Update from 2026-07-07: a disposable receive-only patch was built in the local
OpenRTX checkout and saved as local artifacts:

```text
patch = local artifact (not published)
patch_sha256 = c9435119bc9a1125a1cd49339edd247827ddef3f1ac3975b3240287f6dd452d6
openrtx_worktree = /tmp/adhr-openrtx-isolated/OpenRTX
patch_diff_check = pass
build_verification = pass
builder_image = adhr-openrtx-builder:ubuntu24-miosix9.2
firmware_bin_sha256 = dc432e821ee4ace7bc242a48952a3df48d1a0008971765abacca7a85ba569b59
wrapped_firmware_sha256 = bbbf51dae865ff7e827497635e64f0c7ee1861b8183a0452b926942ca7b507b1
symbol_check_sha256 = 8eb137c0305e325bfa1625a41fbd9e8eefff7ff3cebdcce88a1c1ad1f7675210
```

The patch was accepted as build evidence before the first hardware write. The
write result below is the first hardware checkpoint; it is not an RF capability
claim.

Update from 2026-07-08: the first-boot/rollback checklist is drafted below.
`radio_tool` was built natively for NanoPi/FriendlyWrt and saved under
`/usr/local/bin/radio_tool`; the OpenRTX wrapped artifact was copied to
`/root/adhr-radio/`. Device visibility in firmware update mode passed, and the
wrapped artifact was written with `radio_tool`.

Flash write result from 2026-07-08:

```text
radio_tool_list = [0483:df11] idx=000 AnyRoad Technology Digital Radio in USB mode
radio_tool_info = TYT MD-UV380
write_command = /usr/local/bin/radio_tool -d 0 -f -i /root/adhr-radio/openrtx_mduv3x0_adhr_rxonly_wrap
flash_log = local artifact (not published)
flash_log_sha256 = 784e0249e2b024ccbf69e558ed7644000e4ae3c4b61c9cb503b318f2990e88e9
write_result = Done
first_boot = confirmed_by_usb_cdc_and_visual_ui
```

First boot confirmation from 2026-07-08:

```text
post_flash_usb = 0483:5740 STMicroelectronics STM32 Virtual ComPort in FS Mode
tty = /dev/ttyACM0
dfu_mode_after_power_cycle = false
visual_screen = screen lit, frequency near 430 MHz, signal and volume indicators visible
passive_serial_read_3s = no output
passive_serial_read_5s = no output
normal_mode_radio_tool_list = no devices
stock_usb_shell_or_live_host_control = not found
usb_cdc_role = vcom for XMODEM external-flash backup/restore and selected logging
xmodem_receiver = tools/xmodem-rx/xmodem_rx.c
xmodem_receiver_nanopi_sha256 = 433053122ed06622b65e59354b05153656ec68d27705c75a22805086274e8711
xmodem_backup_attempt = timeout_waiting_for_radio_backup_start
xmodem_backup_bytes = 0
xmodem_backup_ui_path = not exposed in current main menu
xmodem_backup_start_condition_if_patched = PTT
ui_photo_evidence = label photos (not published)
ui_screens_observed = Info, About
info_values_visible = battery about 7.44V, RSSI about -88dBm, VHF+UHF, VHF 136-174, UHF 400-480
backup_restore_visible_in_user_photos = false
receive_only_observation_144425 = RSSI rises sharply on Quansheng FM test transmission while TYT screen shows M17 #ALL
receive_only_observation_144425_status = rf_carrier_seen_audio_not_open_due_to_m17_mode
receive_only_observation_144425_nfm = NFM selected; green RX indicator remains on continuously; no audio reported
receive_only_observation_144425_nfm_status = squelch_threshold_or_audio_path_needs_isolation
receive_only_observation_144425_nfm_fixed = CTCSS disabled and SQL raised; Quansheng transmission opens TYT speaker
receive_only_observation_144425_nfm_fixed_status = vhf_analog_nfm_rx_confirmed
receive_only_observation_433525_nfm = Quansheng CH3 PVT-U1 transmission opens TYT speaker on 433.5250
receive_only_observation_433525_nfm_status = uhf_analog_nfm_rx_confirmed
ptt_tested = false
rf_tx_tested = false
field_tx_ready = false
```

Receive-only observation result:

```text
2026-07-08 VHF test:
  source = Quansheng UV-K5 V2, analog FM/NFM, 144.4250 MHz
  receiver = TYT MD-UV380, OpenRTX ADHR receive-only patch, NFM
  required settings = CTCSS En. OFF; SQL raised until green RX indicator closes with no signal
  result = speaker/audio opens during Quansheng transmission

2026-07-08 UHF test:
  source = Quansheng CH3 PVT-U1, 433.5250 MHz, scrambler OFF
  receiver = TYT MD-UV380, OpenRTX ADHR receive-only patch, NFM
  result = speaker/audio opens during Quansheng transmission

interpretation = receive-only analog NFM path works on both VHF and UHF
limit = no TYT TX validation; ADHR digital frame/baseband path still not implemented
```

## Current Evidence

Station identity:

```text
model = TYT MD-UV380
power_class = 5W
firmware = D219.043
codeplug_version = V02.37
usb = 0483:df11 AnyRoad Technology Digital Radio in USB mode
```

Read-only artifacts:

```text
local artifact (not published)
conf_sha256 = f1405e5296b282fffd4998f4360c2327f3af40e98a5390b1e52142c5b7caacd1
conf_pair_same = true
raw_img_pair_diff = first 1024 bytes only
```

OpenRTX build baseline:

```text
checkout = /tmp/adhr-openrtx-isolated/OpenRTX
commit = 801fc8dd313eb3ac6f83fd4d3b63ed895effdd56
target = mduv3x0
firmware_bin_bytes = 222312
wrapped_firmware_bytes = 222824
```

## Source Hook Map

| ADHR need | OpenRTX path | Current source evidence |
|---|---|---|
| RX/TX/off interface | `/tmp/adhr-openrtx-isolated/OpenRTX/openrtx/include/interfaces/radio.h` | `radio_enableRx`, `radio_enableTx`, `radio_disableRtx`, `radio_updateConfiguration` |
| MD-UV3x0 RX path | `/tmp/adhr-openrtx-isolated/OpenRTX/platform/drivers/baseband/radio_UV3x0.cpp` | `radio_enableRx()` clears PA/APC and enables AT1846S RX |
| MD-UV3x0 TX path | `/tmp/adhr-openrtx-isolated/OpenRTX/platform/drivers/baseband/radio_UV3x0.cpp` | `radio_enableTx()` configures frequency, C6000 TX audio, AT1846S TX, PA/APC |
| PTT source | `/tmp/adhr-openrtx-isolated/OpenRTX/platform/targets/MD-UV3x0/platform.c` | `platform_getPttStatus()` reads internal and external PTT lines |
| FM mode TX trigger | `/tmp/adhr-openrtx-isolated/OpenRTX/openrtx/src/rtx/OpMode_FM.cpp` | PTT can call `radio_enableTx()` when `txDisable == 0` |
| RTX loop | `/tmp/adhr-openrtx-isolated/OpenRTX/openrtx/src/core/threads.c` | `rtx_threadFunc()` continuously runs `rtx_task()` |
| Runtime reconfigure | `/tmp/adhr-openrtx-isolated/OpenRTX/openrtx/src/rtx/rtx.cpp` | `rtx_task()` applies config and calls `radio_updateConfiguration()` |
| UI-to-RTX TX disable | `/tmp/adhr-openrtx-isolated/OpenRTX/openrtx/src/core/threads.c` | `rtx_cfg.txDisable = state.channel.rx_only \|\| state.txDisable` |

Important implication: the first ADHR patch spike should block TX at more than
one layer. Do not rely only on UI/channel `rx_only`.

## Receive-Only Patch Spike

Goal:

```text
slice_name = adhr_bench_slice_v0
compile_target = openrtx_mduv3x0_bin
default_mode = receive_only
tx_path = hard_blocked
bench_log = stub_or_build_metadata_only
```

Minimum patch behavior:

1. Add a compile-time ADHR bench flag such as `CONFIG_ADHR_BENCH_RX_ONLY`.
2. Force any ADHR bench profile into receive-only mode.
3. Make the FM/M17 PTT-to-TX path refuse TX under the ADHR bench flag.
4. Add a lower-level guard so `radio_enableTx()` cannot energize PA/APC under
   the ADHR bench flag.
5. Emit or preserve enough build metadata to prove which binary was built.
6. Keep all new ADHR data structures bounded and static.
7. Rebuild `openrtx_mduv3x0_bin` and compare size to
   `configs/firmware-contract/openrtx-mduv3x0-resource-budget.json`.

Current disposable patch implements the first guard pass by:

```text
platform/targets/MD-UV3x0/hwconfig.h:
  defines CONFIG_ADHR_BENCH_RX_ONLY

openrtx/src/rtx/rtx.cpp:
  forces rtxStatus.txDisable = 1 at init and after reconfigure

platform/drivers/baseband/radio_UV3x0.cpp:
  adds ADHR_BENCH_RX_ONLY build marker
  returns from radio_enableTx() before PA/APC/TX path can be energized
```

Acceptance evidence after first write:

```text
patched_openrtx_build = pass
binary_size_delta = -672 bytes
wrapped_size_delta = -672 bytes
firmware_bin_sha256 = dc432e821ee4ace7bc242a48952a3df48d1a0008971765abacca7a85ba569b59
wrapped_artifact_sha256 = bbbf51dae865ff7e827497635e64f0c7ee1861b8183a0452b926942ca7b507b1
map_sha256 = f321596aa4455cb9b80cd77ddff58e55688325e1554751563107f4cc3ca95a8b
first_boot_plan_reviewed = true
explicit_user_flash_approval = user_go_ahead_2026_07_08
not_flashed = false
first_boot_result = confirmed_by_usb_cdc_and_visual_ui
```

Non-goals for this patch:

- no ADHR RF transmission;
- no field TX;
- no stock TYT firmware replacement on the connected station;
- no DMR compatibility claim;
- no M17 compatibility claim;
- no voice codec implementation;
- no encryption.

## Size Guard

The spike passes only if it stays inside the planning budget:

```text
flash_addition <= 65536 bytes
smallram_static_addition <= 8192 bytes
largeram_dynamic_addition <= 24576 bytes
no heap allocation in slot-critical code
```

Baseline from the isolated build:

```text
firmware_bin_bytes = 222312
flash_remaining_bytes = 646040
smallram_remaining_bytes = 51624
largeram_heap_stack_remaining_bytes = 89600
```

If the patch exceeds the guard, cut scope before any hardware work.

Measured patched build:

```text
elf_text_bytes = 219916
elf_data_bytes = 1720
elf_bss_bytes = 53160
elf_total_dec_bytes = 274796
firmware_bin_bytes = 221640
wrapped_firmware_bytes = 222152
map_bytes = 880284
marker_in_raw_bin = present
marker_in_wrapped_artifact = absent
```

Symbol check:

```text
adhr_bench_build_marker = 0803b820 T adhr_bench_build_marker
radio_enableTx = 080153d4 <radio_enableTx>: 4770 bx lr
```

The raw `.bin` contains the ADHR build marker. The TYT/OpenRTX wrapped artifact
does not preserve that plain marker string, so the symbol check and SHA-256 are
the accepted build evidence.

## First-Boot / Rollback Checklist

Purpose: make the first OpenRTX boot reversible and receive-only. This checklist
does not authorize a firmware write by itself.

### Exact Artifact

Flash candidate:

```text
path = local artifact (not published)
target = OpenRTX mduv3x0 / TYT MD-UV380 family
wrap_mode = UV3X0
load_address = 0x0800C000
size = 222152 bytes
sha256 = bbbf51dae865ff7e827497635e64f0c7ee1861b8183a0452b926942ca7b507b1
```

Keep the raw `.bin` for build evidence only:

```text
path = local artifact (not published)
sha256 = dc432e821ee4ace7bc242a48952a3df48d1a0008971765abacca7a85ba569b59
```

For `mduv3x0`, the local OpenRTX `meson.build` defines:

```text
wrap = UV3X0
load_addr = 0x0800C000
flash command = radio_tool -d 0 -f -i <wrapped artifact>
```

Therefore the first flash candidate is the wrapped artifact, not a raw DFU
write with `dfu-util`.

### Restore Baseline

Stock restore candidate:

```text
archive = local artifact (not published)
archive_sha256 = 857e66cf2f6975829422ef4d9d172d331b5377e4d5826b052a7730880d46075a
candidate_inside_archive = MD-UV380-AES255(REC)-D219.74.bin
candidate_size = 1360384 bytes
```

Why this family:

```text
physical_label = MD-UV380 / 5W / 136-174MHz / 400-480MHz
radio_ui_firmware = D219.043
radio_ui_cp_version = V02.37
same_family_restore = MD-UV380 AES255 REC D219
```

Do not use `MD-UV390*`, `MD-UV380-10W*`, CSV/V219, or non-AES D19/V19 files
for same-family restore of this physical station.

Existing backups:

```text
codeplug_readbacks = local artifact (not published)
codeplug_verify_readback = local artifact (not published)
latest_live_readback = local artifact (not published)
dfu_backup_archive = local artifact (not published)
```

Limit: the DFU/internal flash uploads are useful evidence but are not accepted
as a proven full stock firmware restore image. The practical rollback baseline
is official TYT firmware plus saved codeplug/configuration artifacts.

### Stop Conditions Before Write

Stop and do not flash if any condition is true:

```text
radio_tool_missing_on_usb_host = true
radio_tool_lists_zero_radios = true
radio_tool_lists_multiple_radios = true
wrapped_artifact_sha256_mismatch = true
restore_archive_missing_or_sha_mismatch = true
stock_restore_family_unclear = true
user_explicit_flash_approval_missing = true
```

The exact approval phrase for a future write should name the file:

```text
Yes, flash the MD-UV380 with the OpenRTX receive-only file openrtx_mduv3x0_adhr_rxonly_wrap
```

### Tooling Gate

Current observed tooling state:

```text
nanopi_radio_tool = built
nanopi_radio_tool_path = /usr/local/bin/radio_tool
nanopi_radio_tool_local_copy = local artifact (not published)
nanopi_radio_tool_sha256 = 51f0d24f5cbaede37db4e741044168738f05ef3ec438361d24605bc63c9a199c
nanopi_radio_tool_build = manual g++ C++20 build from v0l/radio_tool commit 6fe1523
nanopi_radio_tool_binary = aarch64 musl ELF
nanopi_firmware_artifact = /root/adhr-radio/openrtx_mduv3x0_adhr_rxonly_wrap
nanopi_firmware_artifact_sha256 = bbbf51dae865ff7e827497635e64f0c7ee1861b8183a0452b926942ca7b507b1
```

`radio_tool --fw-info` on the planned wrapped artifact reports:

```text
== TYT Firmware ==
Radio: MD-9600 (UV3X0)
Size: 216.45 kiB
Data Segments:
  0: Start=0x0800c000, Length=0x000361c8
```

The `MD-9600 (UV3X0)` label is the radio_tool firmware-family label for this
wrapped segment; the important evidence is `UV3X0` and start address
`0x0800c000`.

Current NanoPi `radio_tool --list` without the radio in firmware update mode
returns no devices. This is expected when the station is not connected.

The next valid read-only gate is to connect the MD-UV380 to the NanoPi in
firmware update mode and verify that `radio_tool --list` shows the actual
TYT/UV3X0 device.

If the NanoPi path fails, fallback options:

1. Re-check the same cable/mode that previously exposed `0483:df11` to NanoPi.
2. Use a native host or VM only if USB passthrough is proven with `radio_tool
   --list`.

Read-only checks before write:

```sh
/usr/local/bin/radio_tool --help
/usr/local/bin/radio_tool --list
sha256sum /root/adhr-radio/openrtx_mduv3x0_adhr_rxonly_wrap
```

Expected hash:

```text
bbbf51dae865ff7e827497635e64f0c7ee1861b8183a0452b926942ca7b507b1
```

### Firmware Update Mode Gate

The radio must be in the TYT firmware update/DFU mode expected by
`radio_tool`. Do not rely only on LEDs. The practical positive criterion is:

```text
radio_tool --list shows exactly one supported TYT MD-UV380/MD-UV3x0-class device
```

If only `dfu-util -l` sees `0483:df11` but `radio_tool --list` does not, stop
and fix the tool/mode/cable path before writing.

### Write Command, Not Yet Executed

When all gates pass and the operator gives explicit approval, the intended write
command is:

```sh
/usr/local/bin/radio_tool -d <tyt_index> -f -i /root/adhr-radio/openrtx_mduv3x0_adhr_rxonly_wrap
```

`<tyt_index>` must be the index from a real TYT/UV3X0 firmware-update device,
not a generic serial entry. Do not use an index unless `radio_tool --list`
clearly shows the connected MD-UV380/MD-UV3x0-class device.

### First Boot After Write

First boot is receive-only:

1. Power-cycle the radio normally after the flash tool reports success.
2. Do not press `PTT`.
3. Do not connect external RF test gear for transmission.
4. Confirm that OpenRTX boots to a UI.
5. Record screen text/version if visible.
6. Check that basic UI navigation works.
7. Do not run any TX/RF measurement in this step.

Expected first-boot outcome:

```text
openrtx_boots = true
ptt_tested = false
rf_tx_tested = false
field_tx_ready = false
```

### Rollback Path

If OpenRTX does not boot or the radio must return to stock:

1. Re-enter the TYT firmware update mode.
2. Use the same `radio_tool --list` or official TYT updater visibility gate.
3. Restore only the same-family stock image:

```text
MD-UV380-AES255(REC)-D219.74.bin
```

4. Prefer the official TYT updater/CPS path for stock restore unless
   `radio_tool --fw-info` and a dry tooling check clearly accept the stock
   image.
5. After stock firmware restore, restore or compare the saved codeplug using
   `dmrconfig` artifacts.

Stop rollback if the only available firmware candidate is a different family
(`MD-UV390`, `10W`, `CSV/V219`, or non-AES `D19/V19`).

## Flash Gate

The first receive-only flash write has already consumed this gate:

```text
receive_only_patch_compiles = true
size_guard_pass = true
restore_family_recorded = true
stock_codeplug_artifacts_saved = true
openrtx_wrapped_artifact_hash_recorded = true
first_boot_plan_reviewed = true
radio_tool_usb_host_ready = true
explicit_user_flash_approval = true
field_tx_ready = false
write_result = Done
first_boot = confirmed_by_usb_cdc_and_visual_ui
```

First boot remains receive-only. Do not test PTT or RF transmit in this bench
step.

## Next Actions

1. Power-cycle the MD-UV380 into normal mode after the completed SNAP firmware
   write.
2. Open the normal-mode USB CDC device and send:

```text
ADHR SETNFM 433525000
ADHR SNAP
```

3. Confirm that the forced-state snapshot reports analog NFM:

```text
ADHR RX seq=0 op=1 st=1 bw=0 rx=433525000 tx=433525000 rssi=-88 sql=9 sq=0 tone=0 txdis=1
```

4. During the Quansheng `CH3 PVT-U1` UHF test, issue repeated `ADHR SNAP`
   commands before, during, and after a short Quansheng carrier to capture RSSI
   and squelch state transitions.
5. Repeat the already working receive-only Quansheng tests on `144.4250` and
   `433.5250`, but use point-in-time USB snapshots rather than relying only on
   the streaming telemetry loop.
6. Defer audio capture. External audio capture is noisier and less controlled
   than firmware events for the next ADHR gate.
7. Defer XMODEM external-flash backup unless a later firmware patch exposes a
   safe non-PTT backup path.
8. Do not press `PTT`.
9. Do not run RF/TX measurements.
10. Keep first bench work receive-only until a later lab-safety gate explicitly
   accepts any transmit-capable test.

Telemetry build candidate:

```text
contract = configs/firmware-contract/openrtx-mduv3x0-adhr-telemetry-build-20260708.json
wrapped_firmware = local artifact (not published)
wrapped_sha256 = 8eda5a79619c2c79295a6a58def0d27c95f43d91348579a9e593960a6205c4a0
status = firmware_written_normal_boot_telemetry_verified
safety_check = radio_enableTx remains 4770 bx lr
flash_log = local artifact (not published)
flash_log_sha256 = 59a950b53e6ec3d84a04261342bdb82957eb3fe0a491e22fb6e911c18c860fe3
normal_telemetry_log = local artifact (not published)
normal_telemetry_log_sha256 = e7bb4f4a66a4bc079d47d9bc6270cafa179b14bf8d5a027338d16ed7aab22c39
normal_telemetry_first_line = ADHR RX seq=0 op=2 st=0 bw=0 rx=144425000 tx=144425000 rssi=-137 sql=4 sq=0 tone=0 txdis=1
nfm_capture_attempt_log = local artifact (not published)
nfm_capture_attempt_sha256 = b064dbfee5d4738cd4cf2173ff0cf7274325c3be3b3eab9fe5a5e2ff39e34dec
nfm_capture_attempt_result = blocked; telemetry reported op=2/st=0 instead of analog op=1/st=1
next_gate = stabilize or force analog FM/NFM state before squelch transition telemetry capture
```

SETFM/SETNFM build candidate:

```text
contract = configs/firmware-contract/openrtx-mduv3x0-adhr-setfm-build-20260708.json
wrapped_firmware = local artifact (not published)
wrapped_sha256 = d883400a8bf577e1d1437e2b96822a9925da4ad34f7e73ae329d64492c227369
raw_bin_sha256 = c8d43b7d3b4cee7013559f8be38dbe940167ab4414b795d0cb5fb3664156f5b4
patch = local artifact (not published)
patch_sha256 = 9b69f52ee987d97948854c2debfa2e3429fdd01a53b677c7373818b50098aae4
status = firmware_written_normal_boot_pending
safety_check = radio_enableTx remains 4770 bx lr
radio_enableTx_symbol = 08018660 <radio_enableTx>: 4770 bx lr
flash_log = local artifact (not published)
flash_log_sha256 = 080f09da977c9272cee07a3b905b06e45ca689fd444512e75c9b73089cd402b2
new_commands = ADHR SETNFM <freq_hz>; ADHR SETFM <freq_hz>
frequency_guard = 136000000..480000000 Hz
expected_after_setnfm_144425 = ADHR RX ... op=1 st=1 bw=0 rx=144425000 tx=144425000 ... txdis=1
expected_after_setnfm_433525 = ADHR RX ... op=1 st=1 bw=0 rx=433525000 tx=433525000 ... txdis=1
next_gate = power_cycle_to_normal_mode_then_verify_forced_analog_state_before_squelch_transition_capture
```

SNAP build candidate:

```text
contract = configs/firmware-contract/openrtx-mduv3x0-adhr-snap-build-20260708.json
wrapped_firmware = local artifact (not published)
wrapped_sha256 = b0016960d80e9ed7db55616e0ef7fd61902f8297432630589b3ee228f8c7af7d
patch = local artifact (not published)
patch_sha256 = 707b3539ef9da0a7cc115732ed6448f9a680e02b9482babf20d73c9fb5e0f06b
status = firmware_written_normal_boot_pending
safety_check = radio_enableTx remains 4770 bx lr
radio_enableTx_symbol = 08018660 <radio_enableTx>: 4770 bx lr
flash_log = local artifact (not published)
flash_log_sha256 = a7b2aadd81ea55b7eb4f56d900dbaabdad55ad34bcd8569f63f1553e7628941e
new_commands = ADHR SNAP; ADHR CLEAR; ADHR SETNFM <freq_hz>; ADHR SETFM <freq_hz>
expected_after_setnfm_433525_snap = ADHR RX ... op=1 st=1 bw=0 rx=433525000 tx=433525000 ... txdis=1
next_gate = power_cycle_to_normal_mode_then_verify_ADHR_SETNFM_and_ADHR_SNAP
```
