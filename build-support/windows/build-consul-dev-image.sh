#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


cd ../../
VERSION=1.16.0
docker build -t windows/consul:${VERSION}-dev -f build-support/windows/Dockerfile-consul-dev-windows . --build-arg VERSION=${VERSION}
