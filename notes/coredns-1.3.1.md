+++
title = "CoreDNS-1.3.1 Release"
description = "CoreDNS-1.3.1 Release Notes."
tags = ["Release", "1.3.1", "Notes"]
release = "1.3.1"
date = "2019-01-13T15:00:29+00:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.3.1)
of CoreDNS-1.3.1! This is a fairly small release that allows us to announce some backwards
incompatible changes in the *next* (1.4.0) release:

 *  The `upstream` directive used in various plugin will start to *default* to the coredns process
    itself. This allow those resolutions to take advantage of other plugins (i.e. caching). The
    *etcd*'s plugin StubDomain subsystem relied heavily on this functionality and as such will be
    removed from that plugin.

 *  Multiple endpoints in kubernetes will not be supported going forward.


# Plugins

Mostly documentation updates in various plugins. Plus a small fix where we stop setting the RA
(recursion available) flag on responses in plugins that don't provide recursion.

 *  [*log*](/plugins/log) now allows multiple names to be specified.

 *  [*import*](/plugins/import) was added to give it a README.md to make its documentation more
    discoverable.

 *  [*kubernetes*](/plugins/kubernetes) `TTL` is also applied to negative responses (NXDOMAIN, etc).

## Brought to You By

Chris O'Haver,
ckcd,
Isolus,
jmpcyc,
Miek Gieben,
Taras Tsugrii,
Yong Tang.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
