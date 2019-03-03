+++
title = "CoreDNS-1.1.1 Release"
description = "CoreDNS-1.1.1 Release Notes."
tags = ["Release", "1.1.1", "Notes"]
release = "1.1.1"
date = "2018-03-25T18:04:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.1.1) of
CoreDNS-1.1.1!

This release fixes a **critical bug** in the *cache* plugin found by [Cure53](/2018/03/15/cure53-security-assessment/).

All users are encouraged to upgrade.

## Core

Fix a bug when scrubbing the reply to fit the request's buffer consumes 100% CPU and does not return
the reply.

## Plugins

* [*cache*](/plugins/cache) fixes the critical spoof vulnerability.
* [*route53*](/plugins/route53) adds support for PTR records.

## Contributors

The following people helped with getting this release done:

Chris O'Haver,
Mario Kleinsasser,
Miek Gieben,
Yong Tang.

And of course the people in [Cure53](https://cure53.de). Also special shout out to Mario Kleinsasser
for helping to debug the [scrubbing issue](https://github.com/coredns/coredns/issues/1625).

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
