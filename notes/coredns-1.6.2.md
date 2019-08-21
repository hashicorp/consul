+++
title = "CoreDNS-1.6.2 Release"
description = "CoreDNS-1.6.2 Release Notes."
tags = ["Release", "1.6.2", "Notes"]
release = "1.6.2"
date = 2019-08-13T14:35:47+01:00
author = "coredns"
+++

The CoreDNS team has released
[CoreDNS-1.6.2](https://github.com/coredns/coredns/releases/tag/v1.6.2).

This is a bug fix release, but it also features a new plugin called [*azure*](/plugins/azure).

It's compiled with Go 1.12.8 that incorporates fixes for HTTP/2 that may impact you if you use
[DoH](https://tools.ietf.org/html/rfc8484).

# Plugins

* Add [*azure*](/plugins/azure) to facilitate serving records from Microsoft Azure.
* Make the refresh frequency adjustable in the [*route53*](/plugins/route53) plugin.
* Fix the handling of truncated responses in [*forward*](/plugins/forward).

## Brought to You By

Andrey Meshkov,
Chris O'Haver,
Darshan Chaudhary,
ethan,
Matt Kulka
and
Miek Gieben.

## Noteworthy Changes

* plugin/azure: Add plugin for Azure DNS (https://github.com/coredns/coredns/pull/2945)
* plugin/forward: Fix handling truncated responses in forward (https://github.com/coredns/coredns/pull/3110)
* plugin/kubernetes: Don't do a zone transfer for NS requests (https://github.com/coredns/coredns/pull/3098)
* plugin/kubernetes: fix NXDOMAIN/NODATA fallthough case (https://github.com/coredns/coredns/pull/3118)
* plugin/route53: make refresh frequency adjustable (https://github.com/coredns/coredns/pull/3083)
* plugin/route53: Various updates (https://github.com/coredns/coredns/pull/3108)
