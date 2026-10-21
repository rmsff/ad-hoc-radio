# OpenRTX Isolated Build Helper

This helper builds OpenRTX in a Docker `linux/amd64` container so the Miosix
toolchain is not installed into macOS.

It does not flash a radio and does not make a hardware or RF claim.

```bash
tools/openrtx-build/build-mduv3x0.sh
```

Defaults:

```text
OpenRTX tag = v0.4.4
OpenRTX commit = 801fc8dd313eb3ac6f83fd4d3b63ed895effdd56
target = mduv3x0
cross file = cross_cm4.txt
builder image = adhr-openrtx-builder:ubuntu24-miosix9.2
base image = ubuntu:24.04@sha256:4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90
source checkout = /tmp/adhr-openrtx-isolated/OpenRTX
```

The script verifies that `OpenRTX tag` resolves to `OpenRTX commit` before
building. If the upstream tag moves, the helper fails instead of silently
producing a different resource baseline.

Outputs stay in the temporary OpenRTX checkout:

```text
build_cm4_adhr/openrtx_mduv3x0
build_cm4_adhr/openrtx_mduv3x0_bin
build_cm4_adhr/openrtx_mduv3x0_wrap
build_cm4_adhr/main.map
```

Do not commit generated firmware artifacts.
