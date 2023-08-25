#!/bin/bash
set -e

openssl req -new -sha256 -nodes -out ../key/ourdomain.csr -newkey rsa:2048 -keyout ../key/ourdomain.key -config ../key/ourdomain.cfg
openssl ca -batch -config myca.conf -notext -in ../key/ourdomain.csr -out ../key/ourdomain.cer
rm ../key/ourdomain.csr
