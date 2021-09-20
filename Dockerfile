FROM debian:bullseye-slim

RUN apt-get update && \
  apt-get -y install --no-install-recommends \
    ca-certificates \
    curl

RUN curl https://packages.microsoft.com/keys/microsoft.asc