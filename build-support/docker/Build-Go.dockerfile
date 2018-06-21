ARG GOLANG_VERSION=1.10.1
FROM golang:${GOLANG_VERSION}

ARG GOTOOLS="github.com/elazarl/go-bindata-assetfs/... \
   github.com/hashicorp/go-bindata/... \
   github.com/magiconair/vendorfmt/cmd/vendorfmt \
   github.com/mitchellh/gox \
   golang.org/x/tools/cmd/cover \
   golang.org/x/tools/cmd/stringer \
   github.com/axw/gocov/gocov \
   gopkg.in/matm/v1/gocov-html"

RUN go get -u -v ${GOTOOLS} && mkdir -p ${GOPATH}/src/github.com/hashicorp/consul

WORKDIR $GOPATH/src/github.com/hashicorp/consul

