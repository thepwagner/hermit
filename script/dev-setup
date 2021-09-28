#!/bin/bash

set -e

DEST="${1:-/usr/local/bin}"

RUNC_VERSION="v1.0.2"
RUNC_SHASUM="44d1ba01a286aaf0b31b4be9c6abc20deab0653d44ecb0d93b4d0d20eac3e0b6"

BUILDKIT_VERSION="v0.9.0"
BUILDKIT_SHASUM="1b307268735c8f8e68b55781a6f4c03af38acc1bc29ba39ebaec6d422bccfb25"

FIRECRACKER_VERSION="v0.24.6"
FIRECRACKER_SHASUM="b6c28a30819dffc0c4dc39337ab220decd9f26d9533d118f389f9ba2c2cf375f"

fetch() {
  local url="$1"
  local path="$2"
  local sha="$3"

  echo "$sha  $path" | sha256sum -c - && return 0
  curl -Lo "$path" "$url"
  echo "$sha  $path" | sha256sum -c -
}

download_runc() {
  fetch "https://github.com/opencontainers/runc/releases/download/${RUNC_VERSION}/runc.amd64" \
    "$DEST/runc" \
    "$RUNC_SHASUM"
  chmod +x "$DEST/runc"
}

download_buildkit() {
  local BUILDKIT_FILE="/tmp/buildkit.tgz"
  fetch "https://github.com/moby/buildkit/releases/download/${BUILDKIT_VERSION}/buildkit-${BUILDKIT_VERSION}.linux-amd64.tar.gz" \
    "$BUILDKIT_FILE" \
    "$BUILDKIT_SHASUM"
  tar -xzvvf "$BUILDKIT_FILE" -C "$DEST" --strip-components=1
  cat<<EOF >"/etc/systemd/system/buildkit.socket"
[Unit]
Description=BuildKit

[Socket]
ListenStream=%t/buildkit/buildkitd.sock
SocketMode=0660
SocketUser=root
SocketGroup=adm

[Install]
WantedBy=sockets.target
EOF
  cat<<EOF >"/etc/systemd/system/buildkit.service"
[Unit]
Description=BuildKit
Requires=buildkit.socket
After=buildkit.socket

[Service]
ExecStart=/usr/local/bin/buildkitd --addr fd://

[Install]
WantedBy=multi-user.target
EOF
}

download_firecracker() {
  local FIRECRACKER_FILE="/tmp/firecracker.tgz"
  fetch "https://github.com/firecracker-microvm/firecracker/releases/download/${FIRECRACKER_VERSION}/firecracker-${FIRECRACKER_VERSION}-x86_64.tgz" \
    "$FIRECRACKER_FILE" \
    "$FIRECRACKER_SHASUM"
  tar -xvvzf "$FIRECRACKER_FILE" -C "$DEST" --strip-components=1 \
    "release-${FIRECRACKER_VERSION}/firecracker-${FIRECRACKER_VERSION}-x86_64" \
    "release-${FIRECRACKER_VERSION}/jailer-${FIRECRACKER_VERSION}-x86_64"
  mv "${DEST}/firecracker-${FIRECRACKER_VERSION}-x86_64" "${DEST}/firecracker"
  chmod +x "${DEST}/firecracker"
  mv "${DEST}/jailer-${FIRECRACKER_VERSION}-x86_64" "${DEST}/jailer"
  chmod +x "${DEST}/jailer"
}

download_runc
download_buildkit
download_firecracker

systemctl daemon-reload
systemctl restart buildkit.service
systemctl restart buildkit.socket
