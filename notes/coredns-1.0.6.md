+++
title = "CoreDNS-1.0.6 Release"
description = "CoreDNS-1.0.6 Release Notes."
tags = ["Release", "1.0.6", "Notes"]
draft = false
release = "1.0.6"
date = "2018-02-21T11:10:29+00:00"
author = "coredns"
enabled = "default"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.0.6) of CoreDNS-1.0.6!
This release has bug fixes, documentation fixes, polish and new plugins.

## Core

We've moved to a OWNERS model, where each plugin (and CoreDNS itself) now has an OWNERS file listing
people involved with this code.

## Plugins

* The *startup* and *shutdown* plugin are **deprecated** (but working and included) in this release in favor of the *on*
  plugin. If you use them, this is the moment to move to [*on*](/explugins/on).
* A plugin called [*forward*](https://coredns.io/plugins/forward) has been included in CoreDNS, this
  was, up until now, an external plugin. Supports DNS-over-TLS and has different way of health
  checking an upstream.
* The [*proxy*](https://coredns.io/plugins/proxy) plugin has a new policy, *first* which always
  chooses the first healthy upstream host. It also contains an important fix where
  a non-health checked target could be mark unhealthy forever.
* We now support zone transfers in the [*kubernetes*](https://coredns.io/plugins/kubernetes) plugin.
* The [*bind*](https://coredns.io/plugins/bind) now supports multiple listening addresses.
* Bugfixes, improvements and documentation fixes in various other plugins.

## Contributors

The following people helped with getting this release done:

Chris O'Haver,
Francois Tur,
Freddy,
Harshavardhana,
John Belamaric,
Miek Gieben,
Pat Moroney,
Paul Greenberg,
Sandeep Rajan,
Tobias Schmidt,
Uladzimir Trehubenka,
Yong Tang.

For documentation and help, see our [community page](https://coredns.io/community/).
