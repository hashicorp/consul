#!/usr/bin/env bash

# Create a temp dir and clean it up on exit
TEMPDIR=`mktemp -d -t consul-test.XXX`
trap "rm -rf $TEMPDIR" EXIT HUP INT QUIT TERM

# Build the Consul binary for the API tests
echo "--> Building consul"
go build -o $TEMPDIR/consul || exit 1

# Run the tests
echo "--> Running tests"
go list ./... | PATH=$TEMPDIR:$PATH xargs -n1 go test
