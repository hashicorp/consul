ARG GOLANG_VERSION=1.12.8
FROM golang:${GOLANG_VERSION}

ARG GOTOOLS="github.com/elazarl/go-bindata-assetfs/... \
   github.com/hashicorp/go-bindata/... \
   github.com/mitchellh/gox \
   golang.org/x/tools/cmd/cover \
   golang.org/x/tools/cmd/stringer \
   github.com/axw/gocov/gocov \
   gopkg.in/matm/v1/gocov-html"

RUN go get -u -v ${GOTOOLS} && mkdir -p /consul

WORKDIR /consul
