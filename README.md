[![CoreDNS](https://coredns.io/images/CoreDNS_Colour_Horizontal.png)](https://coredns.io)

[![Documentation](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/coredns/coredns)
[![Build Status](https://img.shields.io/travis/coredns/coredns/master.svg?label=build)](https://travis-ci.org/coredns/coredns)
[![Code Coverage](https://img.shields.io/codecov/c/github/coredns/coredns/master.svg)](https://codecov.io/github/coredns/coredns?branch=master)
[![Docker Pulls](https://img.shields.io/docker/pulls/coredns/coredns.svg)](https://hub.docker.com/r/coredns/coredns)
[![Go Report Card](https://goreportcard.com/badge/github.com/coredns/coredns)](https://goreportcard.com/report/coredns/coredns)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bhttps%3A%2F%2Fgithub.com%2Fcoredns%2Fcoredns.svg?type=shield)](https://app.fossa.io/projects/git%2Bhttps%3A%2F%2Fgithub.com%2Fcoredns%2Fcoredns?ref=badge_shield)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1250/badge)](https://bestpractices.coreinfrastructure.org/projects/1250)

CoreDNS (written in Go) chains [plugins](https://coredns.io/plugins). Each plugin performs a DNS
function.

CoreDNS is a [Cloud Native Computing Foundation](https://cncf.io) inception level project.

CoreDNS is the successor to [SkyDNS](https://github.com/skynetservices/skydns). SkyDNS is a thin
layer that exposes services in etcd in the DNS. CoreDNS builds on this idea and is a **generic** DNS
server that can talk to multiple backends (etcd, kubernetes, etc.).

CoreDNS aims to be a fast and flexible DNS server. The keyword here is *flexible*: with CoreDNS you
are able to do what you want with your DNS data. And if not: write a plugin!

CoreDNS can listen for DNS request coming in over UDP/TCP (go'old DNS), TLS ([RFC
7858](https://tools.ietf.org/html/rfc7858)) and [gRPC](https://grpc.io) (not a standard).

Currently CoreDNS is able to:

* Serve zone data from a file; both DNSSEC (NSEC only) and DNS are supported (*file*).
* Retrieve zone data from primaries, i.e., act as a secondary server (AXFR only) (*secondary*).
* Sign zone data on-the-fly (*dnssec*).
* Load balancing of responses (*loadbalance*).
* Allow for zone transfers, i.e., act as a primary server (*file*).
* Automatically load zone files from disk (*auto*).
* Caching (*cache*).
* Health checking endpoint (*health*).
* Use etcd as a backend, i.e., a 101.5% replacement for
  [SkyDNS](https://github.com/skynetservices/skydns) (*etcd*).
* Use k8s (kubernetes) as a backend (*kubernetes*).
* Serve as a proxy to forward queries to some other (recursive) nameserver (*proxy*).
* Provide metrics (by using Prometheus) (*metrics*).
* Provide query (*log*) and error (*error*) logging.
* Support the CH class: `version.bind` and friends (*chaos*).
* Support the RFC 5001 DNS name server identifier (NSID) option (*nsid*).
* Profiling support (*pprof*).
* Rewrite queries (qtype, qclass and qname) (*rewrite*).
* Echo back the IP address, transport and port number used (*whoami*). This is also the default
  plugin that gets loaded when CoreDNS can't find a Corefile to load.

Each of the plugins has a README.md of its own, see [coredns.io/plugins](https://coredns.io/plugins)
for all in-tree plugins, and [coredns.io/explugins](https://coredns.io/explugins) for all
out-of-tree plugins.

## Status

CoreDNS can be used as an authoritative nameserver for your domains. CoreDNS should be able to
provide you with enough functionality to replace parts of BIND 9, Knot, NSD or PowerDNS and SkyDNS.

## Compilation

Check out the project and do dependency resolution with:

    go get github.com/coredns/coredns

Some of the dependencies require Go version 1.8 or later.

(If you already have the source of CoreDNS checked out in the appropriate place in your `GOPATH`, you can get all
dependencies with `go get ./...`.)

Then use `go build` as you would normally do:

    go build

This should yield a `coredns` binary.

## Compilation with Docker

CoreDNS requires Go to compile. However, if you already have docker installed and prefer not to setup
a Go environment, you could build coredns easily:

```
$ docker run --rm -i -t -v $PWD:/go/src/github.com/coredns/coredns \
      -w /go/src/github.com/coredns/coredns golang:1.9 make
```

The above command alone will have `coredns` binary generated.

## Examples

When starting CoreDNS without any configuration, it loads the
[*whoami*](https://coredns.io/plugins/whoami) plugin and starts listening on port 53 (override with
`-dns.port`), it should show the following:

~~~ txt
.:53
2016/09/18 09:20:50 [INFO] CoreDNS-001
CoreDNS-001
~~~

Any query send to port 53 should return some information; your sending address, port and protocol
used.

If you have a Corefile without a port number specified it will, by default, use port 53, but you
can override the port with the `-dns.port` flag:

`./coredns -dns.port 1053`, runs the server on port 1053.

Start a simple proxy, you'll need to be root to start listening on port 53.

`Corefile` contains:

~~~ corefile
.:53 {
    proxy . 8.8.8.8:53
    log
}
~~~

Just start CoreDNS: `./coredns`. Then just query on that port (53). The query should be forwarded to
8.8.8.8 and the response will be returned. Each query should also show up in the log which is
printed on standard output.

Serve the (NSEC) DNSSEC-signed `example.org` on port 1053, with errors and logging sent to standard
output. Allow zone transfers to everybody, but specifically mention 1 IP address so that CoreDNS can
send notifies to it.

~~~ txt
example.org:1053 {
    file /var/lib/coredns/example.org.signed {
        transfer to *
        transfer to 2001:500:8f::53
    }
    errors
    log
}
~~~

Serve `example.org` on port 1053, but forward everything that does *not* match `example.org` to a recursive
nameserver *and* rewrite ANY queries to HINFO.

~~~ txt
.:1053 {
    rewrite ANY HINFO
    proxy . 8.8.8.8:53

    file /var/lib/coredns/example.org.signed example.org {
        transfer to *
        transfer to 2001:500:8f::53
    }
    errors
    log
}
~~~

IP addresses are also allowed. They are automatically converted to reverse zones:

~~~ corefile
10.0.0.0/24 {
    whoami
}
~~~
Means you are authoritative for `0.0.10.in-addr.arpa.`.

This also works for IPv6 addresses. If for some reason you want to serve a zone named `10.0.0.0/24`
add the closing dot: `10.0.0.0/24.` as this also stops the conversion.

This even works for CIDR (See RFC 1518 and 1519) addressing, i.e `10.0.0.0/25`, CoreDNS will then
check if the `in-addr` request falls in the correct range.

Listening on TLS and for gRPC? Use:

~~~ corefile
tls://example.org grpc://example.org {
    whoami
}
~~~

Specifying ports works in the same way:

~~~ txt
grpc://example.org:1443 {
    # ...
}
~~~

When no transport protocol is specified the default `dns://` is assumed.

## Community

- Website: <https://coredns.io>
- Blog: <https://blog.coredns.io>
- Twitter: [@corednsio](https://twitter.com/corednsio)
- Github: <https://github.com/coredns/coredns>
- Mailing list/group: <coredns-discuss@googlegroups.com>
- Slack: #coredns on <https://slack.cncf.io>

## Deployment

Examples for deployment via systemd and other use cases can be found in the
[deployment repository](https://github.com/coredns/deployment).
