+++
title = "CoreDNS-1.4.0 Release"
description = "CoreDNS-1.4.0 Release Notes."
tags = ["Release", "1.4.0", "Notes"]
release = "1.4.0"
date = "2019-03-03T09:04:07+00:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.4.0)
of CoreDNS-1.4.0! Our first release after we became a graduated project in
[CNCF](https://www.cncf.io/).

Deprecation notice for the *next* release:

 *  [*auto*](/plugins/auto) will deprecate **TIMEOUT** and recommends the use of RELOAD ([2516](https://github.com/coredns/coredns/issues/2516)).
 *  [*auto*](/plugins/file) and [*file*](/plugins/auto) will deprecate NO_RELOAD and recommends the use of RELOAD set to 0 ([2536](https://github.com/coredns/coredns/issues/2536)).
 *  [*health*](/plugins/health) will revert back to report process level health without plugin
    status. A new *ready* plugin will make sure plugins have at least completed their startup
    sequence.
 *  The [*proxy*](/plugins/proxy) will be moved to an external repository and as such be deprecated
    from the default set of plugin; use the [*forward*](/plugins/forward) as a replacement.

The [previous](/2019/01/13/coredns-1.3.1-release/) announced deprecations have been enacted.

The (unused) gRPC watch functionally was removed from the server.

Note we're actively working on two (probably related) bugs
([2593](https://github.com/coredns/coredns/issues/2593),
[2624](https://github.com/coredns/coredns/issues/2624)) which should hopefully result in a fix and
a new release fairly quickly.

# Plugins

Random updates in documentation and fixes in tests and various plugins.

 *  [*kubernetes*](/plugins/kubernetes) fixes the logging now that kubernetes' client lib switched
    to klog from glog.

 *  [*hosts*](/plugins/hosts) fixes IPv4 addresses in IPV6 syntax.

 *  [*etcd*](/plugins/etcd) adds credential support and a fix for the reply when the `host` field is
    empty.

 *  [*log*](/plugins/log) has been made more efficient.

 *  [*forward*](/plugins/forward) drops out of order messages, this is solve occasionally FORMERRs
    people saw.

## Brought to You By

Think we never had so many contributors for a single release. This is really nice to see. Thank you
all:

AdamDang,
Anders Ingemann,
Andrey Meshkov,
Brian Bao,
Carl-Magnus Björkell,
Chris Aniszczyk,
Chris O'Haver,
Christophe de Carvalho,
ckcd,
Dan Kohn,
Darshan Chaudhary,
DO ANH TUAN,
Guillaume Gelin,
Guy Templeton,
JoeWrightss,
Kenjiro Nakayama,
LongKB,
Miek Gieben,
mrasu,
Nguyen Hai Truong,
Nguyen Phuong An,
Nguyen Quang Huy,
Nguyen Van Duc,
Nguyen Van Trung,
Rob Maas,
Ruslan Drozhdzh,
Sandeep Rajan,
Thomas Mangin,
tuanvcw,
Uladzimir Trehubenka,
Xiao An,
Xuanwo,
Ye Ben,
Yong Tang.
