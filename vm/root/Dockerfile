FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y \
  curl \
  systemd-sysv \
  linux-image-amd64

# Kernel is loaded from the host, we only wanted modules
RUN rm -Rf \
  /boot/* \
  /initrd.img /initrd.img.old \
  /vmlinuz vmlinuz.old \
  /lib/modules

ARG CONTAINERD_VERSION="v1.5.5"
ARG CONTAINERD_SHASUM="8efc527ffb772a82021800f0151374a3113ed2439922497ff08f2596a70f10f1"
RUN curl -Lo /tmp/containerd.tgz "https://github.com/containerd/containerd/releases/download/${CONTAINERD_VERSION}/containerd-$(echo $CONTAINERD_VERSION | sed -e 's/^v//g')-linux-amd64.tar.gz" && \
    echo "${CONTAINERD_SHASUM}  /tmp/containerd.tgz" | sha256sum -c - && \
    tar -xzvvf /tmp/containerd.tgz -C "/usr/local/bin" --strip-components=1 && \
    rm -f /tmp/containerd.tgz

ARG RUNC_VERSION="v1.0.2"
ARG RUNC_SHASUM="44d1ba01a286aaf0b31b4be9c6abc20deab0653d44ecb0d93b4d0d20eac3e0b6"
RUN curl -Lo /usr/local/bin/runc "https://github.com/opencontainers/runc/releases/download/${RUNC_VERSION}/runc.amd64" && \
    echo "${RUNC_SHASUM}  /usr/local/bin/runc" | sha256sum -c - && \
    chmod +x /usr/local/bin/runc

ARG BUILDKIT_VERSION="v0.9.0"
ARG BUILDKIT_SHASUM="1b307268735c8f8e68b55781a6f4c03af38acc1bc29ba39ebaec6d422bccfb25"
RUN curl -Lo /tmp/buildkit.tgz "https://github.com/moby/buildkit/releases/download/${BUILDKIT_VERSION}/buildkit-${BUILDKIT_VERSION}.linux-amd64.tar.gz" && \
    echo "${BUILDKIT_SHASUM}  /tmp/buildkit.tgz" | sha256sum -c - && \
    tar -xzvvf /tmp/buildkit.tgz -C "/usr/local/bin" --strip-components=1 && \
    rm -f /tmp/buildkit.tgz

COPY containerd.service /etc/systemd/system/containerd.service
COPY containerd.toml /etc/containerd.toml
RUN systemctl enable containerd.service

COPY buildkit.service /etc/systemd/system/buildkit.service
COPY buildkit.socket /etc/systemd/system/buildkit.socket
RUN systemctl enable buildkit.service

RUN systemctl mask getty@tty2.service
RUN systemctl mask getty@tty3.service
RUN systemctl mask getty@tty4.service
RUN systemctl mask getty@tty5.service
RUN systemctl mask getty@tty6.service
RUN systemctl mask systemd-timesyncd.service
RUN echo "sandbox" > /etc/hostname

# FIXME: testing, not a backdoor i promise
RUN echo 'root:root' | chpasswd