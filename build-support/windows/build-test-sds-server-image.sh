#!/usr/bin/env bash

cd ../../test/integration/connect/envoy

docker build -t windows/test-sds-server -f ./Dockerfile-test-sds-server-windows test-sds-server
