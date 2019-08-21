+++
date = "2017-02-22T21:26:11Z"
description = "CoreDNS-006 Release Notes."
tags = ["Release", "006", "Notes"]
release = "006"
title = "CoreDNS-006 Release"
author = "coredns"
+++

CoreDNS-006 has been [released](https://github.com/coredns/coredns/releases/tag/v006)!

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

# What is New

# Core

Move CoreDNS to <https://github.com/coredns/coredns> together with several other repos. This will be
the new home for CoreDNS development.

Fixed:

* Fix hot-reloading. This would fail with `[ERROR] SIGUSR1: listen tcp :53: bind: address already in
  use`.
* Allow removal of core plugin, see comments in
  [plugin.cfg](https://github.com/miekg/coredns/blob/master/plugin.cfg).

## Plugin improvements

### New

* *reverse* plugin: allows CoreDNS to respond dynamically to an PTR request and the related
  A/AAAA request.

### Improvements/changes

* *proxy* a new `protocol`: `grpc`: speak DNS over gRPC. Server side impl. resides [in this out of
  tree plugin](https://github.com/coredns/grpc).
* *file* additional section processing for MX and SRV queries.
* *prometheus* fix hot reloading
* *trace* various improvements

# Contributors

The following people helped with getting this release done:

John Belamaric,
Miek Gieben,
Richard Hillmann,
Yong Tang,

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
