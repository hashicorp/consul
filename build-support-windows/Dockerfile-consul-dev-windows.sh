#!/usr/bin/env bash

cd ../
rm -rf dist

export GOOS=windows GOARCH=amd64
CONSUL_VERSION=1.12.0
CONSUL_BUILDDATE=$(date +"%Y-%m-%dT%H:%M:%SZ")
GIT_IMPORT=github.com/hashicorp/consul/version
GOLDFLAGS=" -X $GIT_IMPORT.Version=$CONSUL_VERSION -X $GIT_IMPORT.VersionPrerelease=local -X $GIT_IMPORT.BuildDate=$CONSUL_BUILDDATE "

go build -ldflags "$GOLDFLAGS" -o ./dist/ .

docker build -t windows/consul-dev -f ./build-support-windows/Dockerfile-consul-dev-windows .
