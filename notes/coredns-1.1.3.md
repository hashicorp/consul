+++
title = "CoreDNS-1.1.3 Release"
description = "CoreDNS-1.1.3 Release Notes."
tags = ["Release", "1.1.3", "Notes"]
release = "1.1.3"
date = "2018-05-24T09:43:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.1.3) of
CoreDNS-1.1.3!

This release has fixes in the plugins, small core updates and experimental DNS over HTTPs support.
We also announce the deprecation of a few things.

# Core

*Experimental* DNS-over-HTTPS support was added in the server. Use `https://` as the server's scheme
 in the configuration.

The `-log` flag actually doesn't do anything, so this is a deprecation notice that this flag will be
removed in the next release.

# Plugins

* [*metrics*](/plugin/metrics) All in-tree plugins serve metrics with a `server` label.
   * *cache* and *dnssec* drop the capacity metrics
   * add a panic counter: `coredns_panic_count_total`
* [*etcd*](/plugins/etcd) supports A and AAAA record under the zone's apex.
* [*rewrite*](/plugins/rewrite) now handles `continue` in response rewrites.
* [*forward*](/plugin/forward) clean ups, esp when shutting it down.
* [*cache*](/plugin/cache) adds some optimization.
* [*kubernetes*](/plugin/kubernetes) adds option to `ignore` services without ready endpoints.
* Deprecation notice for the *reverse* plugin.
* Deprecation notice for the `https_google` protocol in *proxy*.

## Contributors

The following people helped with getting this release done:

Ahmet Alp Balkan,
Anton Antonov,
Cem Türker,
Chris O'Haver,
darkweaver87,
Eugen Kleiner,
Francois Tur,
John Belamaric,
Mario Kleinsasser,
Miek Gieben,
Ruslan Drozhdzh,
Silver,
Tobias Schmidt,
Yong Tang.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
