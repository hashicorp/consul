FROM alpine:latest
MAINTAINER Miek Gieben <miek@coredns.io> @miekg

# only need ca-certificates & openssl if want to use https_google
RUN apk --update add bind-tools ca-certificates openssl && update-ca-certificates && rm -rf /var/cache/apk/*

ARG COREDNS_VERSION=latest

RUN echo build coredns v$COREDNS_VERSION

ADD https://github.com/coredns/coredns/releases/download/v${COREDNS_VERSION}/coredns_${COREDNS_VERSION}_linux_x86_64.tgz /

RUN tar -xzf coredns_${COREDNS_VERSION}_linux_x86_64.tgz && rm coredns_${COREDNS_VERSION}_linux_x86_64.tgz

EXPOSE 53 53/udp
ENTRYPOINT ["/coredns"]
