+++
title = "CoreDNS-1.2.3 Release"
description = "CoreDNS-1.2.3 Release Notes."
tags = ["Release", "1.2.3", "Notes"]
release = "1.2.3"
date = "2018-10-16T11:37:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.2.3) of
CoreDNS-1.2.3!

## Core

This is a big release that spans almost 6 weeks of development, slightly longer than normal. You may
also have noticed that CoreDNS *wasn't* made the default in Kubernetes 1.12 due to increased memory
used compared to kube-dns. This release contains a fix for that.

The underlying DNS library has seen multiple updates to improve throughput and memory and we have
enabled REUSE_PORT on the ports CoreDNS opens on \*nix.

## Plugins

* [*federation*](/plugins/federation) return a correct answer (SERVFAIL) if availability-zone or region labels are missing from a node.
* [*route53*](/plugins/route53)
    * Refactor add-on to support batch querying of Route 53 along with all AWS record types (including `CNAME`).
    * Add support for zones with overlapping domains (split config)
    * Minor improvements (`fallthrough`, `upstream` options, AWS credentials file support)
* [*cache*](/plugin/cache) add a minttl option to set the minimal TTL for records being cached. The cache key moved to hash/fnv64.
* [*rewrite*](/plugin/rewrite) can now also rewrite TTLs
* [*kubernetes*](/plugin/kubernetes)
    * Uses less memory (~30% less).
    * Do not block on startup when connecting to the API server; returns SERVFAIL in the mean time.
    * Support for using a `kubeconfig` file, including various auth providers (Azure not supported due to a compilation issue with that code).
* [*reload*](/plugin/reload) allows the reload interval to be configured.
* [*forward*](/plugin/forward) fix a crash when health checking is enabled in some circumstances.

## Brought to you by:

Aaron Riekenberg,
Billie Cleek,
Brad Beam,
Can Yucel,
Chris O'Haver,
dilyevsky,
Eugen Kleiner,
Francois Tur,
John Belamaric,
Manuel Alejandro de Brito Fontes,
Manuel Stocker,
marqc,
Miek Gieben,
Nic Cope,
Paul G,
Ruslan Drozhdzh,
Tom Thorogood,
Yong Tang,
Zach Eddy.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
