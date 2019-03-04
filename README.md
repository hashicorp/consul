[![CoreDNS](https://coredns.io/images/CoreDNS_Colour_Horizontal.png)](https://coredns.io)

[![Documentation](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/coredns/coredns)
[![Build Status](https://img.shields.io/travis/coredns/coredns/master.svg?label=build)](https://travis-ci.org/coredns/coredns)
[![Code Coverage](https://img.shields.io/codecov/c/github/coredns/coredns/master.svg)](https://codecov.io/github/coredns/coredns?branch=master)
[![Docker Pulls](https://img.shields.io/docker/pulls/coredns/coredns.svg)](https://hub.docker.com/r/coredns/coredns)
[![Go Report Card](https://goreportcard.com/badge/github.com/coredns/coredns)](https://goreportcard.com/report/coredns/coredns)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1250/badge)](https://bestpractices.coreinfrastructure.org/projects/1250)

CoreDNS (written in Go) chains [plugins](https://coredns.io/plugins). Each plugin performs a DNS
function.

CoreDNS is a [Cloud Native Computing Foundation](https://cncf.io) graduated project.

CoreDNS is a fast and flexible DNS server. The keyword here is *flexible*: with CoreDNS you
are able to do what you want with your DNS data by utilizing plugins. If some functionality is not
provided out of the box you can add it by [writing a plugin](https://coredns.io/explugins).

CoreDNS can listen for DNS requests coming in over UDP/TCP (go'old DNS), TLS ([RFC
7858](https://tools.ietf.org/html/rfc7858)) and [gRPC](https://grpc.io) (not a standard).

Currently CoreDNS is able to:

* Serve zone data from a file; both DNSSEC (NSEC only) and DNS are supported (*file* and *auto*).
* Retrieve zone data from primaries, i.e., act as a secondary server (AXFR only) (*secondary*).
* Sign zone data on-the-fly (*dnssec*).
* Load balancing of responses (*loadbalance*).
* Allow for zone transfers, i.e., act as a primary server (*file*).
* Automatically load zone files from disk (*auto*).
* Caching (*cache*).
* Use etcd as a backend (replace [SkyDNS](https://github.com/skynetservices/skydns)) (*etcd*).
* Use k8s (kubernetes) as a backend (*kubernetes*).
* Serve as a proxy to forward queries to some other (recursive) nameserver (*forward*).
* Provide metrics (by using Prometheus) (*metrics*).
* Provide query (*log*) and error (*errors*) logging.
* Support the CH class: `version.bind` and friends (*chaos*).
* Support the RFC 5001 DNS name server identifier (NSID) option (*nsid*).
* Profiling support (*pprof*).
* Rewrite queries (qtype, qclass and qname) (*rewrite* and *template*).

And more. Each of the plugins is documented. See [coredns.io/plugins](https://coredns.io/plugins)
for all in-tree plugins, and [coredns.io/explugins](https://coredns.io/explugins) for all
out-of-tree plugins.

## Compilation from Source

To compile CoreDNS, we assume you have a working Go setup. See various tutorials if you don’t have that already configured.

First, make sure your golang version is 1.12 or higher as `go mod` support is needed.
See [here](https://github.com/golang/go/wiki/Modules) for `go mod` details.
Then, check out the project and run `make` to compile the binary:
~~~
$ git clone https://github.com/coredns/coredns
$ cd coredns
$ make
~~~

This should yield a `coredns` binary.

## Compilation with Docker

CoreDNS requires Go to compile. However, if you already have docker installed and prefer not to setup
a Go environment, you could build CoreDNS easily:

```
$ docker run --rm -i -t -v $PWD:/go/src/github.com/coredns/coredns \
      -w /go/src/github.com/coredns/coredns golang:1.12 make
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

Any query sent to port 53 should return some information; your sending address, port and protocol
used.

If you have a Corefile without a port number specified it will, by default, use port 53, but you
can override the port with the `-dns.port` flag:

`./coredns -dns.port 1053`, runs the server on port 1053.

Start a simple proxy. You'll need to be root to start listening on port 53.

`Corefile` contains:

~~~ corefile
.:53 {
    forward . 8.8.8.8:53
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
    forward . 8.8.8.8:53

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

This even works for CIDR (See RFC 1518 and 1519) addressing, i.e. `10.0.0.0/25`, CoreDNS will then
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

We're most active on Github (and Slack):

- Github: <https://github.com/coredns/coredns>
- Slack: #coredns on <https://slack.cncf.io>

More resources can be found:

- Website: <https://coredns.io>
- Blog: <https://blog.coredns.io>
- Twitter: [@corednsio](https://twitter.com/corednsio)
- Mailing list/group: <coredns-discuss@googlegroups.com> (not very active)

## Deployment

Examples for deployment via systemd and other use cases can be found in the [deployment
repository](https://github.com/coredns/deployment).

## Deprecation Policy

When there is a backwards incompatible change in CoreDNS the following process is followed:

*  Release x.y.z: Announce that in the next release we will make backward incompatible changes.
*  Release x.y+1.0: Increase the minor version and set the patch version to 0. Make the changes,
   but allow the old configuration to be parsed. I.e. CoreDNS will start from an unchanged
   Corefile.
*  Release x.y+1.1: Increase the patch version to 1. Remove the lenient parsing, so CoreDNS will
   not start if those features are still used.

E.g. 1.3.1 announce a change. 1.4.0 a new release with the change but backward compatible config.
And finally 1.4.1 that removes the config workarounds.

## Security

### Security Audit

A third party security audit was performed by Cure53, you can see the full report [here](https://coredns.io/assets/DNS-01-report.pdf).

### Reporting security vulnerabilities

If you find a security vulnerability or any security related issues, please DO NOT file a public
issue, instead send your report privately to `security@coredns.io`. Security reports are greatly
appreciated and we will publicly thank you for it.

Please consult [security vulnerability disclosures and security fix and release process document](https://github.com/coredns/coredns/blob/master/SECURITY-RELEASE-PROCESS.md)
