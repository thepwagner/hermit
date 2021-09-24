FROM alpine:3.14.2 AS builder

RUN apk --no-cache add \
  alpine-sdk \
  bison \
  curl \
  diffutils \
  elfutils-dev \
  findutils \
  flex \
  linux-headers \
  openssl-dev \
  perl

# Drop privs
WORKDIR /build
RUN chown nobody /build
USER nobody

# Fetch and configure kernel
RUN curl -L https://github.com/torvalds/linux/archive/refs/tags/v5.14.tar.gz | tar -xz
WORKDIR /build/linux-5.14
COPY config .config
RUN make oldconfig

# Compile, go get a coffee
RUN make -j$(nproc) vmlinux

FROM scratch
COPY --from=builder /build/linux-5.14/vmlinux /