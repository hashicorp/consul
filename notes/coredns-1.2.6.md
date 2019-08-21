+++
title = "CoreDNS-1.2.6 Release"
description = "CoreDNS-1.2.6 Release Notes."
tags = ["Release", "1.2.6", "Notes"]
release = "1.2.6"
date = "2018-11-05T20:40:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.2.6) of
CoreDNS-1.2.6!

## Core

Ignore the error when setting SO_REUSEPORT on a socket fails; this makes CoreDNS work on older
kernels.

## Plugins

*  [*etcd*](/plugins/etcd) has seen minor bugfixes.

*  [*loop*](/plugins/loop) fixes a bug when dealing with a failing upstream.

*  [*log*](/plugins/log) unifies all logging (done by this plugin and normal logs) and always use
   RFC3339 timestamps (with millisecond accuracy). The `{when}` verb has been made a noop, it will
   be removed in the next release.

*  [*cache*](/plugins/cache) got some minor optimizations.

*  [*errors*](/plugins/errors) (and *log*) gotten a new option (`consolidate`) to suppress logging.

*  [*hosts*](/plugins/hosts) will now read the `hosts` file without holding a write lock.

*  [*route53*](/plugins/route53) makes the upstream optional.

## Brought to You By

Carl-Magnus Björkell,
Chris O'Haver,
Dzmitry Razhanski,
Francois Tur,
Jiacheng Xu,
Matthias Lechner,
Miek Gieben,
Ruslan Drozhdzh,
Stuart Nelson.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
