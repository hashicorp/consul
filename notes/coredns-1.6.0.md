+++
title = "CoreDNS-1.6.0 Release"
description = "CoreDNS-1.6.0 Release Notes."
tags = ["Release", "1.6.0", "Notes"]
release = "1.6.0"
date = 2019-07-03T07:35:47+01:00
author = "coredns"
+++

The CoreDNS team has released
[CoreDNS-1.6.0](https://github.com/coredns/coredns/releases/tag/v1.6.0).

The `-cpu` flag is removed from this version.

This release sports changes in the *file* plugin. A speed up in the *log* plugin and fixes in the
*cache* plugin.

Upcoming deprecation: the kubernetes *federation* plugin will be moved to
github.com/coredns/federation. This is likely to happen in CoreDNS 1.7.0.

# Plugins

* The [*file*](/plugins/file) a lot of bug fixes and it loads lazily on start, i.e. if the zonefile
  does not exist, it keeps trying with every reload period.
* The [*cache*](/plugins/cache) fixes a race.
* Multiple fixes in the [*route53*](/plugins/route53).
* And the [*kubernetes*](/plugins/kubernetes) removes the `resyncperiod` option.

## Brought to You By

* Wonderful people

## Noteworthy Changes

plugin/file: Simplify locking (https://github.com/coredns/coredns/pull/3024)
plugin/file: New zone should have zero records (https://github.com/coredns/coredns/pull/3025)
plugin/file: Rename do to walk and cleanup and document (https://github.com/coredns/coredns/pull/2987)
plugin/file: Fix setting ReloadInterval (https://github.com/coredns/coredns/pull/3017)
plugin/file: Make non-existent file non-fatal (https://github.com/coredns/coredns/pull/2955)
plugin/metrics: Fix response_rcode_count_total metric (https://github.com/coredns/coredns/pull/3029)
pkg/cache: Fix race in Add() and Evict() (https://github.com/coredns/coredns/pull/3013)
plugin/route53: Fix IAM credential file (https://github.com/coredns/coredns/pull/2983)
plugin/route53: Fix multiple credentials in route53 (https://github.com/coredns/coredns/pull/2859)
pkg/replacer: Evaluate format once and improve perf by ~3x (https://github.com/coredns/coredns/pull/3002)
plugin/log: Fix log plugin benchmark and slightly improve performance (https://github.com/coredns/coredns/pull/3004)
core: Scrub: TC bit is always set (https://github.com/coredns/coredns/pull/3001)
plugin/rewrite: Fix domain length validation (https://github.com/coredns/coredns/pull/2995)
plugin/kubernetes: Remove resyncperiod (https://github.com/coredns/coredns/pull/2923)
