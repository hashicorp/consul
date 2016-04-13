FROM alpine:latest
MAINTAINER Miek Gieben <miek@miek.nl> @miekg

RUN apk --update add bind-tools && rm -rf /var/cache/apk/*

ADD coredns /coredns

EXPOSE 53 53/udp
ENTRYPOINT ["/coredns"]
