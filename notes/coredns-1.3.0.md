+++
title = "CoreDNS-1.3.0 Release"
description = "CoreDNS-1.3.0 Release Notes."
tags = ["Release", "1.3.0", "Notes"]
release = "1.3.0"
date = "2018-12-15T16:14:29+00:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.3.0) of
CoreDNS-1.3.0!

## Core

In this release we do the EDNS0 handling in the server and make it compliant with
[https://dnsflagday.net/](https://dnsflagday.net/). This fits a theme where we move more and more
protocol details into the server to make life easier for plugin authors.

# Plugins

*  [*k8s_external*](/plugins/k8s_external) a new plugin that allows external zones to point to
   Kubernetes in-cluster services.

*  [*rewrite*](/plugins/rewrite) fixes a bug where a rule would eat the first character of a name

*  [*log*](/plugins/log) now supported the [*metadata*](/plugins/metadata) labels. It also fixes a
   bug in the formatting of a plugin logging a info/failure/warning

*  [*forward*](/plugins/forward) removes the dynamic read timeout and uses a fixed value now.

*  [*kubernetes*](/plugins/kubernetes) now checks if a zone transfer is allowed. Also allow a TTL of
   0 to avoid caching in the cache plugin.

## Brought to You By

Chris O'Haver,
Cricket Liu,
Daniel Garcia,
DavadDi,
Francois Tur,
Jiacheng Xu,
John Belamaric,
Miek Gieben,
moredhel,
Sandeep Rajan,
StormXX,
stuart nelson,
Yong Tang.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
