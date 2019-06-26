+++
title = "CoreDNS-1.5.1 Release"
description = "CoreDNS-1.5.1 Release Notes."
tags = ["Release", "1.5.1", "Notes"]
release = "1.5.1"
date = 2019-06-26T13:54:47+01:00
author = "coredns"
+++

The CoreDNS team has released
[CoreDNS-1.5.1](https://github.com/coredns/coredns/releases/tag/v1.5.1).

Various bugfixes, better documentation and cleanups.

The `-cpu` flag is somewhat redundant (cgroups/systemd/GOMAXPROCS are better ways to deal with
this) and we want to remove it; if you depend on it in some way please voice that in [this
PR](https://github.com/coredns/coredns/pull/2793) otherwise we'll remove it in the next release.

# Plugins

* A new plugin [*any*](/plugins/any) that block ANY queries according to [RFC 8482](https://tools.ietf.org/html/rfc8482) was added.
* Failed reload fixes for: [*ready*](/plugins/ready), [*health*](/plugins/health) and
  [*prometheus*](/plugins/metrics) - when CoreDNS reloads and the Corefile is invalid these plugins
  now keep on working. The [*reload*](/plugin/reload) also gained a metric that export failed
  reloads. ([PR](https://github.com/coredns/coredns/pull/2922).
* [*tls*](/plugins/tls) now has a `client_auth` option that allows verification of client TLS certificates. Note that the default behavior continues to be to not require validation, however in version 1.6.0 this default will change to `required_and_verify` if the CA is provided.
* [*kubernetes*](/plugins/kubernetes) can now publish metadata about the request and, if `pods verified` is enabled, about the client Pod. To enable this, you must enable the [*metadata*](/plugins/metadata) plugin.
  And also return pod IPs for running pods, instead of just the first
  ([PR](https://github.com/coredns/coredns/pull/2846) and
  [PR](https://github.com/coredns/coredns/pull/2853)

* The [*cache*](/plugins/cache) now sets the Authoritative bit on replies
  ([PR](https://github.com/coredns/coredns/pull/2885)). Further more it also caches DNS
  failures ([PR](https://github.com/coredns/coredns/pull/2720))

## Brought to You By

Alyx,
Andras Spitzer,
Andrey Meshkov,
Anshul Sharma,
Anurag Goel,
An Xiao,
Billie Cleek,
Chris O'Haver,
Cricket Liu,
Francois Tur,
JINMEI Tatuya,
John Belamaric,
Kun Chang,
Michael Grosser,
Miek Gieben,
Sandeep Rajan,
varyoo,
Yong Tang.

## Noteworthy Changes

* build: Add CircleCI for Integration testing (https://github.com/coredns/coredns/pull/2889)
* core: Add server instance to the context in ServerTLS and ServerHTTPS (https://github.com/coredns/coredns/pull/2840)
* plugin: Add any plugin (https://github.com/coredns/coredns/pull/2801)
* plugin/cache: cache failures (https://github.com/coredns/coredns/pull/2720)
* plugin/cache: remove item.Autoritative (https://github.com/coredns/coredns/pull/2885)
* plugin/chaos: randomize author list (https://github.com/coredns/coredns/pull/2794)
* plugin/health: add OnRestartFailed (https://github.com/coredns/coredns/pull/2812)
* plugin/kubernetes: make ignore empty work with ext svc types (https://github.com/coredns/coredns/pull/2823)
  plugin/kubernetes: never respond with NXDOMAIN for authority label (https://github.com/coredns/coredns/pull/2769)
* plugin/kubernetes: Publish metadata from kubernetes plugin (https://github.com/coredns/coredns/pull/2829)
* plugin/kubernetes: skip deleting pods (https://github.com/coredns/coredns/pull/2853)
* plugin/loop: Update troubleshooting step (https://github.com/coredns/coredns/pull/2804)
  plugin/metrcs: fix datarace on listeners (https://github.com/coredns/coredns/pull/2835)
* plugin/metrics: fix failed reload (https://github.com/coredns/coredns/pull/2816)
* plugin/ready: fix starts and restarts (https://github.com/coredns/coredns/pull/2814)
* plugin/template: Raise error if regexp and template are not specified together (https://github.com/coredns/coredns/pull/2884)
* tls: make sure client CA and auth type are set if CA is explicitly specified. (https://github.com/coredns/coredns/pull/2825)
