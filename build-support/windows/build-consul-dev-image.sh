#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1


cd ../../
rm -rf dist

export GOOS=windows GOARCH=amd64
VERSION=1.16.0
CONSUL_BUILDDATE=$(date +"%Y-%m-%dT%H:%M:%SZ")
GIT_IMPORT=github.com/hashicorp/consul/version
GOLDFLAGS=" -X $GIT_IMPORT.Version=$VERSION -X $GIT_IMPORT.VersionPrerelease=dev -X $GIT_IMPORT.BuildDate=$CONSUL_BUILDDATE "

go build -ldflags "$GOLDFLAGS" -o ./dist/ .

docker build -t windows/consul:${VERSION}-dev -f build-support/windows/Dockerfile-consul-dev-windows . --build-arg VERSION=${VERSION}
