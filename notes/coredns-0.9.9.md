+++
title = "CoreDNS-0.9.9 Release"
description = "CoreDNS-0.9.9 Release Notes."
tags = ["Release", "0.9.9", "Notes"]
draft = false
release = "0.9.9"
date = "2017-10-18T11:37:43-04:00"
author = "coredns"
+++

CoreDNS-0.9.9 has been [released](https://github.com/coredns/coredns/releases/tag/v0.9.9)!
(yes, we've moved to [semver](https://coredns.io/2017/09/16/semantic-versioning/))

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

Release 0.9.9 is a major release, with lots of fixes.

## Core

* We've renamed `middleware.Middleware` to `plugin.Plugin`. This is backwards incompatible for external ~~middleware~~ plugins, but you can update your plugin by just replacing `[Mm]iddleware` with `[Pp]lugin`:
   ~~~
    sed 's/Middleware/Plugin/'g -i *.go
    sed 's/middleware/plugin/'g -i *.go
   ~~~
From now on we'll use the term *plugin* in our documentation and code.

* We've sent a proposal to make CoreDNS the default in Kubernetes: https://github.com/kubernetes/community/pull/1100

## Plugins

* *etcd*'s debug queries are removed.
* *hosts* gets inline host definitions that add or overwrite those from `/etc/hosts`.
* *auto*, *file* now poll every minute for on disk changes (inotify wasn't working).
* *rewrite* can chain rules and perform multiple changes on a message.
* *kubernetes* uses `protobuf` to communicate with the kubernetes API and
performance improvements when there are a large number of services.
* *dnstap* saw several fixes, including sending tap messages out-of-band.
* *cache* apply configured TTL to first answer returned.
   * Don't cache TTL=0 messages.
* *proxy* smaller timeouts and the health check GET was given a timeout.
  * Better metrics: add a request counter metrics and change the 'from' label to 'to' so we count/duration per upstream.
* *dnssec* now signs NODATA responses.

## External Plugins

Two new [external plugins](/explugins) were added:

* *ipecho* parses the IP out of a subdomain and echos it back as an record.
* *forward* facilitates proxying DNS messages to upstream resolvers.

## Contributors

The following people helped with getting this release done:

antonkyrylenko,
Chris O'Haver,
Chris West,
Damian Myerscough,
Isolus,
John Belamaric,
Miek Gieben,
Sandeep Rajan,
Thong Huynh,
varyoo,
Yong Tang.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
