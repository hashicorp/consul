+++
title = "CoreDNS-1.5.0 Release"
description = "CoreDNS-1.5.0 Release Notes."
tags = ["Release", "1.5.0", "Notes"]
release = "1.5.0"
date = 2019-04-06T08:24:07+00:00
author = "coredns"
+++

A new [release](https://github.com/coredns/coredns/releases/tag/v1.5.0): CoreDNS-1.5.0.

**Two** new plugins in this release: [*grpc*](/plugins/grpc), and [*ready*](/plugins/ready). And
some polish and simplifications in the core server code.

The use of **TIMEOUT** and **no_reload** in [*file*](/plugins/file) and [*auto*](/plugins/auto) have
been fully deprecated. As is the [*proxy*](/explugins/proxy/) plugin.

And a update on two important and active bugs:

* [2593](https://github.com/coredns/coredns/issues/2593) seems to hone in on Docker and/or the
  container environment being a contributing factor.

* [2624](https://github.com/coredns/coredns/issues/2624) is because of TLS session negotiating in
  the *forward* plugin.

# Plugins

* The [*ready*](/plugins/ready) plugin was added that signals a plugin is ready to receive queries. First user is the *kubernetes* plugin.
* The [*grpc*](/plugins/grpc) plugin was added to implement forwarding gRPC. If you were relying on this in the [*proxy*](/explugins/proxy) you can migrate to this one.
* The [*cancel*](/plugins/cancel) plugin was added that adds a context.WithTimeout to each context (but not
  enabled - yet).
* The [*forward*](/plugins/forward) plugin adds dnstap support.
* The [*route53*](/plugins/route53) plugin now supports wildcards.
* The [*pprof*](/plugins/pprof) plugin adds a `block` option that enables the block profiling.
* The [*prometheus*](/plugins/metrics) plugin  adds a `coredns_plugin_enabled` metric that shows which plugins are enabled.
* The [*chaos*](/plugins/chaos) plugin returns the maintainers when queried for "authors.bind TXT CH".
* The `resyncperiod` option in [*kubernetes*](/plugins/kubernetes) now defaults to zero seconds, which disables resyncing.

# Deprecations

* The `resyncperiod` option in [*kubernetes*](/plugins/kubernetes) is deprecated
  and will be made a no-op in 1.6.0 and removed in 1.7.0.

## Brought to You By

Aleks,
Chris O'Haver,
David,
dilyevsky,
Francois Tur,
Iñigo,
Jiacheng Xu,
John Belamaric,
Matt Greenfield,
MengZeLee,
Miek Gieben,
peiranliushop,
Rajveer Malviya,
Ruslan Drozhdzh,
Stefan Budeanu,
Xiao An,
Yong Tang.
