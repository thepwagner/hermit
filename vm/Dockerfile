# This is building a root filesystem for the VM, so it breaks Dockerfile convention and prefers to cache
FROM alpine:3.14.2

RUN apk update
RUN apk add linux-virt
RUN apk add openrc
RUN apk add util-linux
RUN ln -s agetty /etc/init.d/agetty.ttyS0
RUN echo ttyS0 > /etc/securetty
RUN rc-update add agetty.ttyS0 default
RUN rc-update add devfs boot
RUN rc-update add procfs boot
RUN rc-update add sysfs boot
