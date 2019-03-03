+++
date = "2016-09-18T11:40:37+01:00"
description = "CoreDNS-001 Release Notes."
release = "001"
tags = ["Release", "001", "Notes"]
title = "CoreDNS-001 Release"
author = "coredns"
+++

CoreDNS-001 has been [released](https://github.com/coredns/coredns/releases). This is the first
release! It provides a complete DNS server, that also does DNSSEC and is useful for service
discovery in cloud setups.

# What is CoreDNS?

[CoreDNS](https://coredns.io) is a DNS server that started its life as a fork of the [Caddy
web(!)server](https://caddyserver.com).

It chains [plugin](https://github.com/coredns/coredns/tree/master/plugin),
where each plugin implements some DNS feature. CoreDNS is a complete replacement
(with more features, and maybe less bugs) for [SkyDNS](https://github.com/skynetservices/skydns).

It is also useful as a normal DNS server, featuring DNSSEC, on-the-fly signing and zone transfers.

# What is New

CoreDNS is now a (the first!) server type plugin in Caddy - this means we can leverage a lot of code
from Caddy without having to fork (and maintain) it all. By doing so we were able to remove 9000
lines of code from CoreDNS.

The core (ghe!) of CoreDNS is now in a good shape. Future work will focus on making the
plugin better, e.g. the proxy implementation is slow and needs to be
[faster](https://github.com/coredns/coredns/issues/184).

## New Plugins

* There is now a [specific
  plugin](https://github.com/coredns/coredns/tree/master/plugin/kubernetes) to deal with [Kubernetes](https://kubernetes.io).
* The *bind* [plugin](https://github.com/coredns/coredns/tree/master/plugin/bind)  allows you to bind to a specific IP address, instead of using the wildcard
  address.
* A *whoami* [plugin](https://github.com/coredns/coredns/tree/master/plugin/whoami) reports
  back your address and port.
* All other plugins are reworked to fit in the new plugin framework from Caddy version 0.9 (and
  up).

The *whoami* plugin is also used when CoreDNS starts up and can't find a Corefile.

# Contributors

The following people helped with getting this release done:

Cricket Liu, elcore, Félix Cantournet, Ilya Dmitrichenko, Joe Blow, Lee, Matt Layher,
Michael Richmond, Miek Gieben, pixelbender, Yong Tang.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/) and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
