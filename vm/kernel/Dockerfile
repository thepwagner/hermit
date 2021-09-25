# Build an uncompressed Debian kernel
FROM debian:bullseye-slim AS builder

RUN echo "deb-src http://deb.debian.org/debian bullseye main" >> /etc/apt/sources.list && \
  echo "deb-src http://security.debian.org/debian-security bullseye-security main" >> /etc/apt/sources.list
RUN apt-get update && \
  apt-get install -y build-essential fakeroot && \
  apt-get build-dep -y linux

# Drop privs
WORKDIR /build
RUN chown nobody /build
USER nobody

# Fetch and configure kernel
RUN apt-get source linux
WORKDIR /build/linux-5.10.46
COPY config .config
RUN make oldconfig

# Compile, grab a coffee
RUN make -j$(nproc) vmlinux

FROM scratch
COPY --from=builder /build/linux-5.10.46/vmlinux /