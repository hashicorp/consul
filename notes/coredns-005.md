+++
date = "2017-02-09T18:50:31Z"
description = "CoreDNS-005 Release Notes."
tags = ["Release", "005", "Notes"]
release = "005"
title = "CoreDNS-005 Release"
author = "coredns"
+++

CoreDNS-005 has been [released](https://github.com/coredns/coredns/releases/tag/v005)!

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

# What is New

# Core

A way to configure (external) plugin was added. Edit `plugin.cfg` and do a `go generate && go
build` and your plugin has been added. This allows for out-of-tree plugin to be easily
added. Documentation can be found in
[plugin.cfg](https://github.com/coredns/coredns/blob/master/plugin.cfg).

## Plugin improvements

### New

* *erratic*: a new plugin that can drop queries, limited in the current functionality, but useful for testing.
* *trace*: a new plugin that implements OpenTracing-based tracing using Zipkin.

### Improvements/changes

* *proxy*: fix a bug when a connection hangs and never gets release (#467)
* *proxy*: Fold *httpproxy* into it, which is now a normal proxy with a special `protocol`. For
  Monitoring an extra label was added: `proxy_proto` that shows the protocol used (`dns` or `https_google`). 
  See the [proxy README.md](https://github.com/coredns/coredns/blob/master/plugin/proxy/README.md) for details.
* *httpproxy*: removed because functionality is moved to *proxy*.
* *kubernetes*: Now implements the full
  [Kubernetes DNS Specification](https://github.com/kubernetes/dns/blob/master/docs/specification.md),
  including regular and headless services, endpoint hostnames, A, SRV, and PTR records.
* *kubernetes*: Implements the `pod` type for requests in both a Kube-DNS compatible mode
  (`insecure`) and a mode which validates that the IP in question belongs to a pod in the specified
  namespace (`verified`)
* *kubernetes*: Simplified the configuration of reverse zones. Instead of listing the zones in the
  zone list, you can just add a list of CIDRs using the `cidrs` option.
* *rewrite*: allow rewriting more bits of the incoming packet. This required some backward
  *incompatible* changes, e.g. a new **FIELD** keyword is now required. See the 
  [rewrite README.md](https://github.com/coredns/coredns/blob/master/plugin/rewrite/README.md) for details.


# Contributors

The following people helped with getting this release done:

Bob Wasniak,
Chris O'Haver,
devnev,
Dmytro Kislov,
John Belamaric,
Miek Gieben,
Yong Tang.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
