+++
title = "CoreDNS-1.1.2 Release"
description = "CoreDNS-1.1.2 Release Notes."
tags = ["Release", "1.1.2", "Notes"]
release = "1.1.2"
date = "2018-04-23T09:21:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.1.2) of
CoreDNS-1.1.2!

This release has some fixes in the plugins and no core updates.

# Plugins

* [*forward*](/plugins/forward) has received a large pile of fixes and improvements.
* [*reload*](/plugins/reload): the *metrics* and *health* plugin saw fixes for this reload issue, still not 100% perfect, but a whole lot better than it was.
* [*log*](/plugins/log) now allows `OR`ing of log classes.
* [*metrics*](/plugins/metrics): add a server label to make each metric unique to the server handling it.
  This impacts all plugins, currently *proxy* and *forward* have been updated to include a server label.
* [*debug*](/plugins/debug): when enabled plugins show their `log.Debug` output (none of the included plugins use this yet).
* [*kubernetes*](/plugins/kubernetes) has a small fix for apex queries.

## Contributors

The following people helped with getting this release done:

Chris O'Haver,
Francois Tur,
Maksim Paramonau,
Miek Gieben,
Moto Ishizawa,
Ruslan Drozhdzh,
Scott Donovan,
Tobias Schmidt,
Uladzimir Trehubenka.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
