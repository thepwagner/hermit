#!/bin/sh
set -ex

export http_proxy=http://127.0.0.1:3128
export https_proxy=http://127.0.0.1:3128

cd /root
git clone https://github.com/thepwagner/archivist.git 
cd archivist
/usr/local/bin/guest build

tar -tvvf "/tmp/image.tar"