FROM alpine:latest
MAINTAINER Miek Gieben <miek@miek.nl> @miekg

# only need ca-certificates & openssl if want to use https_google
RUN apk --no-cache add bind-tools ca-certificates openssl && update-ca-certificates

ADD coredns /coredns

EXPOSE 53 53/udp
ENTRYPOINT ["/coredns"]
