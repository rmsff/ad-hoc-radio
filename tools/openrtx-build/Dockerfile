FROM ubuntu:24.04@sha256:4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90

ARG MIOSIX_URL=https://miosix.org/toolchain/MiosixToolchainInstaller9.2.0mp3.2.run
ARG MIOSIX_SHA256=d71fdac28a6fc4a4c0b41e90fb46c3722805b04e1ddb355d0da2f1b67e50ef3e

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        cmake \
        curl \
        file \
        git \
        libusb-1.0-0-dev \
        meson \
        ninja-build \
        pkg-config \
        python3 \
        python3-setuptools \
        python3-wheel \
        build-essential \
        xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "${MIOSIX_URL}" -o /tmp/miosix-toolchain.run \
    && echo "${MIOSIX_SHA256}  /tmp/miosix-toolchain.run" | sha256sum -c - \
    && mkdir -p /opt/arm-miosix-eabi \
    && sh /tmp/miosix-toolchain.run \
        --target /opt/arm-miosix-eabi \
        --noexec \
        --nox11 \
        --noprogress \
        --nochown \
    && rm -f /tmp/miosix-toolchain.run

ENV PATH="/opt/arm-miosix-eabi/bin:${PATH}"

WORKDIR /src
