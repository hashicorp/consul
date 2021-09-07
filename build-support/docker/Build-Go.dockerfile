ARG GOLANG_VERSION=1.18.1
FROM golang:${GOLANG_VERSION}

ARG GOTOOLS="golang.org/x/tools/cmd/cover \
   golang.org/x/tools/cmd/stringer \
   github.com/axw/gocov/gocov \
   gopkg.in/matm/v1/gocov-html"

RUN mkdir -p .gotools && \
    cd .gotools && \
    for tool in ${GOTOOLS}; do \
        echo "=== TOOL: ${tool}" ; \
        rm -rf go.mod go.sum ; \
        go mod init consul-tools ; \
        go get -v "${tool}" ; \
    done && \
    rm -rf go.mod go.sum
RUN go install github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs@master
RUN go install github.com/hashicorp/go-bindata/go-bindata@master

WORKDIR /consul
