+++
title = "CoreDNS-1.0.5 Release"
description = "CoreDNS-1.0.5 Release Notes."
tags = ["Release", "1.0.5", "Notes"]
draft = false
release = "1.0.5"
date = "2018-01-25T11:10:29+00:00"
author = "coredns"
enabled = "default"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.0.5) of CoreDNS-1.0.5!
This release has bug fixes, documentation fixes, polish and new plugins.

## Core

Add ability to *really* compile out the default plugins.

## Plugins

* A new plugin *route53* was added that enables serving zone data from AWS route53, see the [documentation](https://coredns.io/plugins/route53).
* A new plugin *on* was added. This is an external Caddy [plugin](https://caddyserver.com/docs/on), that is now also available (by default) for CoreDNS; it allows you to run commands when an event is generated.

* *cache* doesn't apply a 5s minimum TTL anymore. It fixes prefetching *and* correctly sets the metrics for cache hits and misses.
* *dnssec* fixes handing out *expired* signatures after 8 days and properly filters out the qtype in the NSEC bitmap for NXDOMAIN responses.
* *log* adds message ID `{>id}` to the default logging.
* *health* has gotten a lameduck option that will nack health, but will keep the server running for a configurable duration when CoreDNS is being shut down. If metrics are enabled *health* exports a metric that curls the local endpoint and exports the duration. Useful for getting a sense of overloadedness of the process.
* *rewrite* can now rewrite answers for `name regex` matches. This prevents DNS clients from ignoring the answers due to a mismatch with the original question.
* *secondary* saw a bunch of fixes.

## Contributors

The following people helped with getting this release done:

Christian Nilsson,
cricketliu,
Francois Tur,
Ilya Galimyanov,
Miek Gieben,
Paul Greenberg,
Ruslan Drozhdzh,
Tobias Schmidt,
Yong Tang,
Yue Ko.

If you want to help, please check out one of the
[issues](https://github.com/coredns/coredns/issues/) and start coding! For documentation and help,
see our [community page](https://coredns.io/community/).
