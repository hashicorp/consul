# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

# CI Dockerfile: build from source using the Go toolchain image.
# For local development where Docker Hub pulls are blocked by credential
# helpers, use Dockerfile.local (selected automatically by run-tests.sh
# when LOCAL_BUILD=1).
FROM golang:latest

WORKDIR /go/src
COPY ./ .

RUN go build -v -o test-sds-server sds.go

CMD ["/go/src/test-sds-server"]
