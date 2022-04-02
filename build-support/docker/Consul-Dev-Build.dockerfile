ARG CONSUL_IMAGE_VERSION=latest
FROM golang:1.17-alpine As builder
RUN apk update && apk add iptables make bash git go
RUN rm -rf ./bin/consul
WORKDIR /consul
COPY . .
RUN make dev
FROM consul:${CONSUL_IMAGE_VERSION}
COPY --from=builder /consul/bin/consul  /bin/consul