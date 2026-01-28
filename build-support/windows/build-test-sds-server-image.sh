#!/usr/bin/env bash
# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1


cd ../../test/integration/connect/envoy

docker build -t windows/test-sds-server -f ./Dockerfile-test-sds-server-windows test-sds-server
