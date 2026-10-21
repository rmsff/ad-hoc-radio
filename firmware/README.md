# Firmware experiments

Receive-only ADHR experiments for the TYT MD-UV380. Background, results and
safety notes: [docs/hardware.md](../docs/hardware.md).

| Directory | Contents | License |
| --- | --- | --- |
| `openrtx-mvp10.3/` | Patch, ADHR overlay, isolated build scripts and evidence for [OpenRTX](https://github.com/OpenRTX/OpenRTX) | GPL-3.0-or-later ([LICENSE](openrtx-mvp10.3/LICENSE)) |
| `opengd77-mvp10.4/`, `opengd77-mvp10.5/`, `opengd77-mvp10.7/` | This project's own `adhr_*` sources (ADHR mode, 27-byte Layer-2 envelope, USB bench), host tests and the host-side USB bench tool | Apache-2.0 |

The `adhr_*` sources build only on top of [OpenGD77](https://www.opengd77.com/),
which is not included. The OpenGD77 license forbids commercial use; any firmware
image built with OpenGD77 falls under that license. The patches and build
tooling that embed OpenGD77 source lines are not distributed; the integration
points are described in [docs/hardware.md](../docs/hardware.md).

Host tests compile only this project's code:

```bash
bash firmware/opengd77-mvp10.4/run-host-tests.sh
bash firmware/opengd77-mvp10.5/run-host-tests.sh
bash firmware/opengd77-mvp10.7/run-host-tests.sh
bash firmware/openrtx-mvp10.3/run-host-tests.sh
```

No vendor firmware, radio backups or flashing images are included. The
firmware here is receive-only; never transmit without a dummy load or the right
to use the band.
