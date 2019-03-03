+++
title = "CoreDNS-011 Release"
description = "CoreDNS-011 Release Notes."
tags = ["Release", "011", "Notes"]
draft = false
release = "011"
date = "2017-09-10T20:24:43-04:00"
author = "coredns"
+++

CoreDNS-011 has been [released](https://github.com/coredns/coredns/releases/tag/v011)!

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

Release v011 is a major release, with backwards incompatible changes in the *kubernetes* plugin.

## Core

**This release has backwards incompatible changes** for the *kubernetes* plugin.

* Stop vendoring `github.com/miekg/dns` and `golang.org/x/net/context`. This enables external plugin to compile without tripping over vendored types that mismatch.
* Allow an easy way to specify reverse zones in the Corefile, just use (e.g) `10.0.0.0/24` as the zone name,
  CoreDNS translates this to 0.0.10.in-addr.arpa. This is only done when the netmask is a multiple of 8 and for both IPv4 and IPv6.
* Bug and stability fixes.

## Plugins

Make *kubernetes*, *file*, *secondary*, *hosts*, *erratic* and *metrics* now fail on unknown properties in the Corefile.

### New

* *federation*: enables federation via kubernetes.
* *autopath*: enables autopath-ing. Can be used standalone, but its main use is with kubernetes.

### Updates

* *log* adds an `>rflags` replacer that shows the flags from the response - this has been enabled by default.
* *kubernetes* deprecates:
   * `cidr`: use the reverse syntax in the Corefile
   * `federation`: use the new *federation* plugin
   * `autopath`: use the new *autopath* plugin
* *kubernetes*:
   * add TTL option allowing to set minimal TTL for responses.
   * Multiple k8s API endpoints could be specified, separated by `","`s, e.g. `endpoint http://k8s-endpoint1:8080,http://k8s-endpoint2:8080`. CoreDNS will automatically perform a healthcheck and proxy to the healthy k8s API endpoint.
* *rewrite*:
   * allow for *dynamic* properties to be used, like client IP address in rewrite rules, i.e.
`rewrite edns0 local set 0xffee {client_ip}`
   * add support for EDNS0 Client Subnet
* *dnstap* now reports messages proxied by *proxy*, and support remote IP endpoints by specifying `tcp://`.
* *dnssec* now warns if keys can't be used to sign the configured zones.
* *health* now allows for per plugin health status; no plugin makes use of this yet, though.
* *secondary* parses a secondary with a zone (`secondary example.org {...}`) correctly.

## Contributors

The following people helped with getting this release done:

Brad Beam,
Chris O'Haver,
insomniac,
James Mills,
John Belamaric,
Markus Sommer,
Miek Gieben
Mohammed Naser,
Sandeep Rajan,
Thong Huynh,
varyoo,
Yong Tang,
张勋.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
