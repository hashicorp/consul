+++
title = "CoreDNS-1.0.1 Release"
description = "CoreDNS-1.0.1 Release Notes."
tags = ["Release", "1.0.1", "Notes"]
draft = false
release = "1.0.1"
date = "2017-12-11T14:43:43-00:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.0.1) of CoreDNS-1.0.1!

This release fixes a crash in the *file* plugin and has some minor bug fixes for other plugins.
One new plugin was added: *nsid*, that implements [RFC 5001](https://tools.ietf.org/html/rfc5001).

## Plugins
* *file* fixes a crash when an request with a DO bit (pretty much the default) hits an unsigned zone. The default configuration should recover the go-routine, but this is nonetheless serious. *file* received some other fixes when returning (secure) delegations.
* *dnstap* plugin is now 50% faster.
* *metrics* fixed the start time bucket for the duration.

## Contributors

The following people helped with getting this release done:
Brad Beam,
James Hartig,
Miek Gieben,
Rene Treffer,
Ruslan Drozhdzh,
Seansean2,
Yong Tang.

If you want to help, please check out one of the
[issues](https://github.com/coredns/coredns/issues/) and start coding! For documentation and help,
see our [community page](https://coredns.io/community/).
