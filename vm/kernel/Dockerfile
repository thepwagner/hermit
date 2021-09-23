FROM debian:bullseye-slim

RUN echo "deb-src http://deb.debian.org/debian bullseye main" >> /etc/apt/sources.list
RUN echo "deb-src http://security.debian.org/debian-security bullseye-security main" >> /etc/apt/sources.list
RUN apt-get update
RUN apt-get install -y build-essential fakeroot
RUN apt-get build-dep -y linux

WORKDIR /build
RUN chown nobody /build
USER nobody
RUN apt-get source linux

COPY config /build/linux-5.10.46/.config
WORKDIR /build/linux-5.10.46
RUN make oldconfig
RUN make -j8 vmlinux
