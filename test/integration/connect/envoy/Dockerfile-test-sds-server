FROM golang:latest

WORKDIR /go/src
COPY ./ .

RUN go build -v -o test-sds-server sds.go

CMD ["/go/src/test-sds-server"]
