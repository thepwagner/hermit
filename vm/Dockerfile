FROM alpine:3.14.2

RUN apk --no-cache add \
  curl \
  openrc \
  util-linux

RUN ln -s agetty /etc/init.d/agetty.ttyS0
RUN echo ttyS0 > /etc/securetty
RUN rc-update add agetty.ttyS0 default
RUN rc-update add devfs boot
RUN rc-update add procfs boot
RUN rc-update add sysfs boot

ARG CONTAINERD_VERSION="v1.5.5"
ARG CONTAINERD_SHASUM="8efc527ffb772a82021800f0151374a3113ed2439922497ff08f2596a70f10f1"

RUN curl -Lo /tmp/containerd.tgz "https://github.com/containerd/containerd/releases/download/${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION:1}-linux-amd64.tar.gz" && \
    echo "${CONTAINERD_SHASUM}  /tmp/containerd.tgz" | sha256sum -c - && \
    tar -xzvvf /tmp/containerd.tgz -C "/usr/local/bin" --strip-components=1
COPY containerd.initd /etc/init.d/containerd
RUN rc-update add containerd boot
