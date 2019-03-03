+++
date = "2017-07-13T22:52:11Z"
release = "009"
description = "CoreDNS-009 Release Notes."
tags = ["Release", "009", "Notes"]
title = "CoreDNS-009 Release"
author = "coredns"
+++

CoreDNS-009 has been [released](https://github.com/coredns/coredns/releases/tag/v009)!

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

Release v009 is mostly a bugfix release, with a few new features in the plugin.

# Core

No changes.

# Plugins

* *secondary*: fix functionality and improve matching of notify queries.
* *cache*: fix data race.
* *proxy*: async healthchecks.
* *reverse*: new option `wildcard` that also catches all subdomains of a template.
* *kubernetes*: experimental new option `autopath` that optimizes the search path and ndots
  combinatorial explosion, so clients with a large search path and high ndots will get a reply on
  the first query.

# Contributors

The following people helped with getting this release done:

Athir Nuaimi,
Chris O'Haver,
ghostflame,
jremond,
Mia Boulay,
Miek Gieben,
Ning Xie,
Roman Mazur,
Yong Tang.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
