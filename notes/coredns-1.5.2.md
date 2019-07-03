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

* For all plugins the `upstream` directive was removed from the documentation; it's still accepted
  but is a noop.
* The [*file*](/plugins/file) closes the connection after an AXFR. It also loads secondary zones
  lazily on startup.

## Brought to You By

bcebere,
JINMEI Tatuya,
Miek Gieben,
Timoses,
Yong Tang.


## Noteworthy Changes

* plugin/file: close correctlty after AXFR (https://github.com/coredns/coredns/pull/2943)
* plugin/file: load secondary zones lazily on startup (https://github.com/coredns/coredns/pull/2944)
* build: Update Caddy to 1.0.1, and update import path (https://github.com/coredns/coredns/pull/2961)
* plugins: set upstream unconditionally (https://github.com/coredns/coredns/pull/2956)
* tls: hardening (https://github.com/coredns/coredns/pull/2938)
