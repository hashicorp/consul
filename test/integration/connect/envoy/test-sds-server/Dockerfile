# Copyright IBM Corp. 2024, 2026
# SPDX-License-Identifier: BUSL-1.1

FROM golang:1.26.5

WORKDIR /go/src
COPY ./ .

RUN go build -v -o test-sds-server sds.go

CMD ["/go/src/test-sds-server"]
