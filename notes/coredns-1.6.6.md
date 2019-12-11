+++
title = "CoreDNS-1.6.6 Release"
description = "CoreDNS-1.6.6 Release Notes."
tags = ["Release", "1.6.6", "Notes"]
release = "1.6.6"
date = 2019-12-11T10:00:00+00:00
author = "coredns"
+++

The CoreDNS team has released
[CoreDNS-1.6.6](https://github.com/coredns/coredns/releases/tag/v1.6.6).

A fairly small release that polishes various plugins and fixes a bunch of bugs.


# Security 

The github.com/miekg/dns has been updated to v1.1.25 to fix a DNS related security
vulnerability (https://github.com/miekg/dns/issues/1043).

# Plugins

A new plugin [*bufsize*](/plugin/bufsize) has been added that prevents IP fragmentation
for the DNS Flag Day 2020 and to deal with DNS vulnerabilities.

* [*cache*](/plugin/cache) added a `serve_stale` option similar to `unbound`'s `serve_expired`.
* [*sign*](/plugin/sign) fix signing of authoritative data that we are not authoritative for.
* [*transfer*](/plugin/transfer) fixed calling wg.Add in main goroutine to avoid race conditons.

## Brought to You By

Chris O'Haver
Gonzalo Paniagua Javier
Guangming Wang
Kohei Yoshida
Miciah Dashiel Butler Masters
Miek Gieben
Yong Tang
Zou Nengren

## Noteworthy Changes

* plugin/bufsize: A new bufsize plugin to prevent IP fragmentation and DNS Flag Day 2020 (https://github.com/coredns/coredns/pull/3401)
* plugin/transfer: Fixed calling wg.Add in main goroutine to avoid race conditions (https://github.com/coredns/coredns/pull/3433)
* plugin/pprof: Fixed a reloading issue (https://github.com/coredns/coredns/pull/3454)
* plugin/health: Fixed a reloading issue (https://github.com/coredns/coredns/pull/3473)
* plugin/redy: Fixed a reloading issue (https://github.com/coredns/coredns/pull/3473)
* plugin/cache: Added a `serve_stale` option similar to `unbound`'s `serve_expired` (https://github.com/coredns/coredns/pull/3468)
* plugin/sign: Fix signing of authoritative data (https://github.com/coredns/coredns/pull/3479)
* pkg/reuseport: Move the core server listening functions to a new package (https://github.com/coredns/coredns/pull/3455)
