+++
title = "CoreDNS-1.2.0 Release"
description = "CoreDNS-1.2.0 Release Notes."
tags = ["Release", "1.2.0", "Notes"]
release = "1.2.0"
date = "2018-07-11T11:13:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.2.0) of
CoreDNS-1.2.0!

In this release we have a new plugin, bump etcd to version 3 and bugfixes.

# Core

Enable watch functionality when CoreDNS is used as a gRPC server (documented in the code - for now).

# Plugins

* A new plugin called [*metadata*](/plugins/metadata) was added. It adds metadata to a query, via the context.
* The [*etcd*](/plugins/etcd) plugin now supports etcd version 3 (only!). It was impossible to support v2 *and* v3 at
  the same time (even as separate plugins); so we decided to drop v2 support.
* Fix a race/crash in the [*cache*](/plugins/cache) plugin when `prefetch` is enabled.
* The [*forward*](/plugins/forward) plugin has a `prefer_udp` option, that even when the incoming query is over TCP, the
  outgoing one will be tried over UDP first.
* [*secondary*](/plugins/secondary) plugin has a bug fix for zone expiration: don't expire zones if we can reach the
  primary, but see no zone changes.
* The [*auto*](/plugins/auto) plugin now works better with Kubernetes Configmaps.

## Contributors

The following people helped with getting this release done:
Chris O'Haver,
Eren Güven,
Eugen Kleiner,
Francois Tur,
Isolus,
Joey Espinosa,
John Belamaric,
Jun Li,
Marcus André,
Miek Gieben,
Nitish Tiwari,
Ruslan Drozhdzh,
Tobias Schmidt,
Yong Tang.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
