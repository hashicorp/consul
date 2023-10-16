#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


cd ../../test/integration/connect/envoy

docker build -t windows/test-sds-server -f ./Dockerfile-test-sds-server-windows test-sds-server
