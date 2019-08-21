+++
date = "2016-11-11T16:38:32Z"
description = "CoreDNS-003 Release Notes."
release = "003"
tags = ["Release", "003", "Notes"]
title = "CoreDNS-003 Release"
author = "coredns"
+++

CoreDNS-003 has been [released](https://github.com/coredns/coredns/releases)!

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

# What is New

## Core

Refused queries are properly logged and exported if metrics are enabled.

## Plugin improvements

* *proxy*: allow  `/etc/resolv.conf` to be used in the configuration.
* *metrics*: add tests and normalize some of the metrics. Removed the AXFR size metrics.
* *cache*: Added size and capacity of the cache (for both `denial` and `success` cache types).
  Don't cache meta data records and zone transfers.
* *dnssec*: metrics were unused, hooked them up: export size and capacity of the signature cache.
* *loadbalance*: balance MX records as well.
* *auto*: numerous bugfixes.
* *file*: fix data race in reload process and also reload a zone when it is `mv`ed (newly created) into place.
  Also rewrite the zone lookup algorithm and be more standards compliant, esp. in the area of DNSSEC, wildcards and empty-non-terminals; handle secure delegations.
* *kubernetes*: vendor the k8s dependency and updates to be compatible with Kubernetes 1.4 and 1.5.
   Multiple cleanups and fixes. Kubernetes services can now be resolved.

# Contributors

The following people helped with getting this release done:

Ben Kochie,
Chris O'Haver,
John Belamaric,
Jonathan Dickinson,
Manuel Alejandro de Brito Fontes,
Michael Grosser,
Miek Gieben,
Yong Tang.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
