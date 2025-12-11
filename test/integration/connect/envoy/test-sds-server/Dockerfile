# Copyright IBM Corp. 2014, 2025
# SPDX-License-Identifier: BUSL-1.1

FROM golang:latest

WORKDIR /go/src
COPY ./ .

RUN go build -v -o test-sds-server sds.go

CMD ["/go/src/test-sds-server"]
