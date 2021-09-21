FROM debian:bullseye-slim@sha256:8aa2e47f9a6cf001ecf3ad0f8439e1005743a3024b98e7bbf023ace55afea903

RUN apt-get update && \
  apt-get -y install --no-install-recommends \
    ca-certificates \
    curl
RUN update-ca-certificates

RUN curl https://packages.microsoft.com/keys/microsoft.asc
