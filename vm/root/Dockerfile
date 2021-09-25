# The end goal is a root filesystem for booting as a VM

# First step: compile the guest go binary
FROM golang:1.17.1 AS builder
WORKDIR /app
COPY go.mod go.sum /app/
RUN go mod download
COPY . .
RUN go build -o /guest ./cmd/guest

# For the final image, start with a debian image:
FROM debian:bullseye-slim@sha256:e3ed4be20c22a1358020358331d177aa2860632f25b21681d79204ace20455a6

# Install an init system and minimize footprint:
RUN apt-get update && \
  apt-get install -y \
    curl \
    systemd-sysv \
    linux-image-amd64 \
  && \
  apt-get clean && \
  rm -Rf \
    /var/lib/apt/lists/* \
    /boot/* \
    /lib/modules
RUN systemctl mask serial-getty@ttyS0.service
RUN systemctl mask getty@tty1.service && \
  systemctl mask getty@tty2.service && \
  systemctl mask getty@tty3.service && \
  systemctl mask getty@tty4.service && \
  systemctl mask getty@tty5.service && \
  systemctl mask getty@tty6.service && \
  systemctl mask systemd-timesyncd.service && \
  echo "sandbox" > /etc/hostname

# Install buildkit:
ARG CONTAINERD_VERSION="v1.5.5"
ARG CONTAINERD_SHASUM="8efc527ffb772a82021800f0151374a3113ed2439922497ff08f2596a70f10f1"
RUN curl -Lo /tmp/containerd.tgz "https://github.com/containerd/containerd/releases/download/${CONTAINERD_VERSION}/containerd-$(echo $CONTAINERD_VERSION | sed -e 's/^v//g')-linux-amd64.tar.gz" && \
    echo "${CONTAINERD_SHASUM}  /tmp/containerd.tgz" | sha256sum -c - && \
    tar -xzvvf /tmp/containerd.tgz -C "/usr/local/bin" --strip-components=1 \
      bin/containerd \
      bin/containerd-shim-runc-v2 \
    && \
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
    tar -xzvvf /tmp/buildkit.tgz -C "/usr/local/bin" --strip-components=1 \
      bin/buildkitd \
    && \
    rm -f /tmp/buildkit.tgz
COPY vm/root/containerd.service /etc/systemd/system/containerd.service
COPY vm/root/containerd.toml /etc/containerd.toml
RUN systemctl enable containerd.service
COPY vm/root/buildkit.service /etc/systemd/system/buildkit.service
COPY vm/root/buildkit.socket /etc/systemd/system/buildkit.socket
RUN systemctl enable buildkit.service

# FIXME: testing, not a backdoor i promise
RUN echo 'root:root' | chpasswd
COPY vm/root/Dockerfile.test /root/Dockerfile

# Install the guest:
COPY vm/root/guestproxy.service /etc/systemd/system/guestproxy.service
COPY vm/root/guestproxy-cert.service /etc/systemd/system/guestproxy-cert.service
COPY vm/root/guestbuild.service /etc/systemd/system/guestbuild.service
RUN systemctl enable guestproxy.service
RUN systemctl enable guestproxy-cert.service
RUN systemctl enable guestbuild.service

COPY --from=builder /guest /usr/local/bin/guest
