+++
title = "CoreDNS-1.0.4 Release"
description = "CoreDNS-1.0.4 Release Notes."
tags = ["Release", "1.0.4", "Notes"]
draft = false
release = "1.0.4"
date = "2018-01-18T15:54:29+00:00"
author = "coredns"
+++

We are announcing the [release](https://github.com/coredns/coredns/releases/tag/v1.0.4) of CoreDNS-1.0.4!

This is a release that fixes a vulnerability in the underlying DNS library.
See <https://github.com/miekg/dns/issues/627> and the (still embargoed) CVE-2017-15133.
Thanks to Tom Thorogood for bringing this issue to our attention.

CoreDNS-1.0.4 is [CoreDNS-1.0.3](https://coredns.io/2018/01/10/coredns-1.0.3-release/) recompiled with a patched DNS library.
