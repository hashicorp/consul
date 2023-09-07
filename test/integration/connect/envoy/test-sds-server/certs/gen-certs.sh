#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


set -eEuo pipefail
unset CDPATH

# force the script to first switch to the directory containing the script before
# messing with the filesystem
cd "$(dirname "$0")"
rm -rf *.crt *.key

openssl genrsa -out ca-root.key 4096
openssl req -x509 -new -nodes -key ca-root.key -out ca-root.crt \
  -subj "/C=US/ST=CA/O=/CN=SDS Test CA Cert" \
  -sha256 -days 3650

function gen_cert {
  local FILE_NAME=$1
  local DNS_NAME=$2

  openssl genrsa -out "$FILE_NAME.key" 2048
  openssl req -new -key "$FILE_NAME.key" -out "$FILE_NAME.csr" \
    -reqexts SAN \
    -config <(cat /etc/ssl/openssl.cnf \
        <(printf "\n[SAN]\nsubjectAltName=DNS:$DNS_NAME")) \
    -subj "/C=US/ST=CA/O=/CN=$DNS_NAME"

  openssl x509 -req -in "$FILE_NAME.csr" \
    -CA ca-root.crt -CAkey ca-root.key -CAcreateserial \
    -out "$FILE_NAME.crt"  -days 3650 -sha256 \
    -extfile <(printf "subjectAltName=DNS:$DNS_NAME")

  rm "$FILE_NAME.csr"
}

DOMAINS="www.example.com foo.example.com *.ingress.consul"

for domain in $DOMAINS
do
  # * in file names is interpreted as a global and all sorts of things go
  #   strange!
  FILE_NAME="$domain"
  if [ ${domain:0:2} == "*." ]; then
    FILE_NAME="wildcard.${domain:2}"
  fi
  gen_cert $FILE_NAME $domain
done