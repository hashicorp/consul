+++
title = "CoreDNS-1.2.4 Release"
description = "CoreDNS-1.2.4 Release Notes."
tags = ["Release", "1.2.4", "Notes"]
release = "1.2.4"
date = "2018-10-17T20:01:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.2.4) of
CoreDNS-1.2.4!

Remember we [said the 1.2.3 release](/2018/10/16/coredns-1.2.3-release/) was a big release and took
quite a while? Well, we've fixed that glitch; as 1.2.4 is here now.

CoreDNS v1.2.3's *kubernetes* plugin **DOES NOT WORK IN KUBERNETES** and our testing that didn't catch
that regression, nor the Kubernetes scale testing which doesn't really exercise the *whole* API.

## Plugins

* [*cache*](/plugins/cache) use zero of the minimal negative TTL (if no suitable TTL was found in
  the packet).
* [*kubernetes*](/plugins/kubernetes) fix a grave bug that made plugin **unusable** in Kubernetes.

## Brought to you by:

Chris O'Haver,
Miek Gieben.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
