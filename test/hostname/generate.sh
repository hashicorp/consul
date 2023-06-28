#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail


# server.dc1.consul
if [[ ! -f Alice.crt ]] || [[ ! -f Alice.key ]]; then
    echo "Regenerating Alice.{crt,key}..."
    rm -f Alice.crt Alice.key
    openssl req -new -sha256 -nodes -out Alice.csr -newkey rsa:2048 -keyout Alice.key -config Alice.cfg
    openssl ca -batch -config myca.conf -extfile Alice.ext -notext -in Alice.csr -out Alice.crt
    rm -f Alice.csr
fi

# bob.server.dc1.consul
if [[ ! -f Bob.crt ]] || [[ ! -f Bob.key ]]; then
    echo "Regenerating Bob.{crt,key}..."
    rm -f Bob.crt Bob.key
    openssl req -new -sha256 -nodes -out Bob.csr -newkey rsa:2048 -keyout Bob.key -config Bob.cfg
    openssl ca -batch -config myca.conf -extfile Bob.ext -notext -in Bob.csr -out Bob.crt
    rm -f Bob.csr
fi

# betty.server.dc2.consul
if [[ ! -f Betty.crt ]] || [[ ! -f Betty.key ]]; then
    echo "Regenerating Betty.{crt,key}..."
    rm -f Betty.crt Betty.key
    openssl req -new -sha256 -nodes -out Betty.csr -newkey rsa:2048 -keyout Betty.key -config Betty.cfg
    openssl ca -batch -config myca.conf -extfile Betty.ext -notext -in Betty.csr -out Betty.crt
    rm -f Betty.csr
fi

# bonnie.server.dc3.consul
if [[ ! -f Bonnie.crt ]] || [[ ! -f Bonnie.key ]]; then
    echo "Regenerating Bonnie.{crt,key}..."
    rm -f Bonnie.crt Bonnie.key
    openssl req -new -sha256 -nodes -out Bonnie.csr -newkey rsa:2048 -keyout Bonnie.key -config Bonnie.cfg
    openssl ca -batch -config myca.conf -extfile Bonnie.ext -notext -in Bonnie.csr -out Bonnie.crt
    rm -f Bonnie.csr
fi
