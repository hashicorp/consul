#!/usr/bin/env bash
set -e

# Create a temp dir and clean it up on exit
TEMPDIR=`mktemp -d -t consul-test.XXX`
trap "rm -rf $TEMPDIR" EXIT HUP INT QUIT TERM

# Build the Consul binary for the API tests
echo "--> Building consul"
go build -tags="${BUILD_TAGS}" -o $TEMPDIR/consul || exit 1

# Run the tests
echo "--> Running tests"
go list ./... | grep -v '/vendor/' | PATH=$TEMPDIR:$PATH xargs -n1 go test -tags="${BUILD_TAGS}" ${GOTEST_FLAGS:--cover -timeout=360s}
