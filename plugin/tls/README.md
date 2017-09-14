# tls

*tls* allows you to configure the server certificates for the TLS and gRPC servers.
For other types of servers it is ignored.

CoreDNS supports queries that are encrypted using TLS (DNS over Transport Layer Security, RFC 7858)
or are using gRPC (https://grpc.io/, not an IETF standard). Normally DNS traffic isn't encrypted at
all (DNSSEC only signs resource records).

The *proxy* plugin also support gRPC (`protocol gRPC`), meaning you can chain CoreDNS servers
using this protocol.

The *tls* "plugin" allows you to configure the cryptographic keys that are needed for both
DNS-over-TLS and DNS-over-gRPC. If the `tls` directive is omitted, then no encryption takes place.

The gRPC protobuffer is defined in `pb/dns.proto`. It defines the proto as a simple wrapper for the
wire data of a DNS message.

## Syntax

~~~ txt
tls CERT KEY CA
~~~

## Examples

Start a DNS-over-TLS server that picks up incoming DNS-over-TLS queries on port 5553 and uses the
nameservers defined in `/etc/resolv.conf` to resolve the query. This proxy path uses plain old DNS.

~~~
tls://.:5553 {
	tls cert.pem key.pem ca.pem
	proxy . /etc/resolv.conf
}
~~~

Start a DNS-over-gRPC server that is similar to the previous example, but using DNS-over-gRPC for
incoming queries.

~~~
grpc://. {
	tls cert.pem key.pem ca.pem
	proxy . /etc/resolv.conf
}
~~~

Only Knot DNS' `kdig` supports DNS-over-TLS queries, no command line client supports gRPC making
debugging these transports harder than it should be.

## Also See

RFC 7858 and https://grpc.io.
