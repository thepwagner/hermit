FROM alpine:3.14.2

RUN apk update
RUN apk add alpine-sdk bison curl flex 

WORKDIR /build
RUN chown nobody /build
USER nobody
RUN curl -L https://github.com/torvalds/linux/archive/refs/tags/v5.14.tar.gz | tar -xz

WORKDIR /build/linux-5.14
COPY config .config
RUN make oldconfig
 RUN make -j8 vmlinux
