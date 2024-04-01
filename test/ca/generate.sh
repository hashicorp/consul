#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -e

openssl req -new -sha256 -nodes -out ../key/ourdomain.csr -newkey rsa:2048 -keyout ../key/ourdomain.key -config ../key/ourdomain.cfg
openssl ca -batch -config myca.conf -notext -in ../key/ourdomain.csr -out ../key/ourdomain.cer
rm ../key/ourdomain.csr

openssl req -new -sha256 -nodes -out ../key/ourdomain_server.csr -newkey rsa:2048 -keyout ../key/ourdomain_server.key -config ../key/ourdomain_server.cfg
openssl ca -batch -config myca.conf -notext -in ../key/ourdomain_server.csr -out ../key/ourdomain_server.cer -extensions v3_req -extfile ../key/ourdomain_server.cfg
rm ../key/ourdomain_server.csr
