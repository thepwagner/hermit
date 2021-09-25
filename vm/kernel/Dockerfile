# Build an uncompressed Debian kernel
FROM debian:bullseye-slim AS builder

# Get build dependencies
RUN echo "deb-src http://deb.debian.org/debian bullseye main" >> /etc/apt/sources.list && \
  echo "deb-src http://security.debian.org/debian-security bullseye-security main" >> /etc/apt/sources.list && \
  apt-get update && \
  apt-get install -y build-essential fakeroot && \
  apt-get build-dep -y linux

# Drop privs
WORKDIR /build
RUN chown nobody /build
USER nobody

# Get and configure kernel sources
RUN apt-get source linux
WORKDIR /build/linux-5.10.46
COPY config .config
RUN make oldconfig

# Compile (grab a coffee)
RUN make -j$(nproc) vmlinux && \
  mv vmlinux /

FROM scratch
COPY --from=builder /vmlinux /