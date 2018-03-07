FROM alpine:latest

# Only need ca-certificates & openssl if want to use DNS over TLS (RFC 7858).
RUN apk --no-cache add bind-tools ca-certificates openssl && update-ca-certificates

ADD coredns /coredns

EXPOSE 53 53/udp
ENTRYPOINT ["/coredns"]
