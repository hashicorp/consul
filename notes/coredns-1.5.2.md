+++
title = "CoreDNS-1.5.2 Release"
description = "CoreDNS-1.5.2 Release Notes."
tags = ["Release", "1.5.2", "Notes"]
release = "1.5.2"
date = 2019-07-03T07:35:47+01:00
author = "coredns"
+++

The CoreDNS team has released
[CoreDNS-1.5.2](https://github.com/coredns/coredns/releases/tag/v1.5.2).

Small bugfixes and a change to Caddy's important path.

# Plugins

* For all plugins that use the `upstream` directive it use removed from the documentation; it's still accepted
  but is a noop. Currently these plugins use CoreDNS to resolve external queries.
* The [*template*](/plugins/template) plugin now supports meta data.
* The [*file*](/plugins/file) plugin closes the connection after an AXFR. It also loads secondary zones
  lazily on startup.

## Brought to You By

bcebere,
John Belamaric,
JINMEI Tatuya,
Miek Gieben,
Timoses,
Yong Tang.

## Noteworthy Changes

* plugin/file: close correctlty after AXFR (https://github.com/coredns/coredns/pull/2943)
* plugin/file: load secondary zones lazily on startup (https://github.com/coredns/coredns/pull/2944)
* plugin/template: support metadata (https://github.com/coredns/coredns/pull/2958)
* build: Update Caddy to 1.0.1, and update import path (https://github.com/coredns/coredns/pull/2961)
* plugins: set upstream unconditionally (https://github.com/coredns/coredns/pull/2956)
* tls: hardening (https://github.com/coredns/coredns/pull/2938)
