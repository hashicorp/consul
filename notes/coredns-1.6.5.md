+++
title = "CoreDNS-1.6.5 Release"
description = "CoreDNS-1.6.5 Release Notes."
tags = ["Release", "1.6.5", "Notes"]
release = "1.6.5"
date = 2019-11-06T10:00:00+00:00
author = "coredns"
+++

The CoreDNS team has released
[CoreDNS-1.6.5](https://github.com/coredns/coredns/releases/tag/v1.6.5).

A fairly small release that polishes various plugins and fixes a bunch of bugs.

# Plugins

A new plugin [*transfer*](/plugins/transfer) that encapsulates the zone transfer knowledge and code
in one place. This makes it easier to add this functionality to new plugins. Plugins that already
implement zone transfers are expected to move to it in the 1.7.0 release.

* [*forward*](/plugins/forward) don't block on returning sockets; instead timeout and drop the
  socket on the floor, this makes each go-routine guarantee to exit.
* [*kubernetes*](/plugins/kubernetes) adds metrics to measure kubernetes control plane latency, see
  documentation for details.
* [*file*](/plugins/file) fixes a panic when comparing domains names.

## Brought to You By

Chris O'Haver,
Erfan Besharat,
Hauke Löffler,
Ingo Gottwald,
janluk,
Miek Gieben,
Uladzimir Trehubenka,
Yong Tang,
yuxiaobo96.

## Noteworthy Changes

* core: Make request.Request smaller (https://github.com/coredns/coredns/pull/3351)
* pkg/log: Add Clear to stop debug logging (https://github.com/coredns/coredns/pull/3372)
* plugin/cache: move goroutine closure to separate function to save memory (https://github.com/coredns/coredns/pull/3353)
* plugin/clouddns: remove initialization from init (https://github.com/coredns/coredns/pull/3349)
* plugin/erratic: doc and zone transfer (https://github.com/coredns/coredns/pull/3340)
* plugin/file: fix panic in miekg/dns.CompareDomainName() (https://github.com/coredns/coredns/pull/3337)
* plugin/forward: make Yield not block (https://github.com/coredns/coredns/pull/3336)
* plugin/forward: Move map to array (https://github.com/coredns/coredns/pull/3339)
* plugin/kubernetes: Measure and expose DNS programming latency from Kubernetes plugin. (https://github.com/coredns/coredns/pull/3171)
* plugin/route53: Remove amazon initialization from init (https://github.com/coredns/coredns/pull/3348)
* plugin/transfer: Zone transfer plugin (https://github.com/coredns/coredns/pull/3223)
* plugins: Add MustNormalize (https://github.com/coredns/coredns/pull/3385)
