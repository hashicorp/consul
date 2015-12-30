#!/bin/bash
docker run \
        --env CGO_ENABLED=0 \
        --env CONSUL_DEV=1 \
        --rm \
        --tty \
        --volume $(pwd):/go/src/github.com/hashicorp/consul \
        --workdir /go/src/github.com/hashicorp/consul \
    golang:1.5 \
        make
