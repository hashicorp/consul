+++
date = "2017-05-03T19:26:11Z"
description = "CoreDNS-007 Release Notes."
release = "007"
tags = ["Release", "007", "Notes"]
title = "CoreDNS-007 Release"
author = "coredns"
+++

CoreDNS-007 has been [released](https://github.com/coredns/coredns/releases/tag/v007)!

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

# News

CoreDNS is accepted as an inception project by the [CNCF](https://cncf.io)! Which means a lot to us.
See [this blog post](/2017/03/02/why-cncf-for-coredns/) on why we wanted/did this.

Because of this we moved repos: <https://github.com/coredns> is the main overarching repo. There is
an automatic redirect in place from the old repo.

And... We have a new logo! We're also discussion a website redesign for <https://coredns.io> and
this blog.

Back to the release.

# Core

* `ServeDNS` is extended to take a context. This allows (for instance) tracing to start at an earlier entrypoint.
* gRPC and TLS are made first class citizens. See [the zone
  specification](https://github.com/coredns/coredns/blob/master/README.md#zone-specification) on how
  to use it. TL;DR using `grpc://` makes the server talk gRPC. The `tls` directive is used to
  specify TLS certificates.
* Zipkin tracing can be enabled for all plugin.

# Plugins

* *rewrite* now allows you to add or modify EDNS0 local or NSID options. The framework is in place to add additional EDNS0 types in the future.
* *etcd* when no upstreams are defined won't default to using 8.8.8.8, 8.8.4.4; it just does not resolve external names in that case.
* *erratic* now can also delay queries and send queries with Truncated set.
* *metrics* will happily start as many (different) listeners as you want (if you really need that).
* *startup* and *shutdown* allow for command to be run during startup or shutdown. These directly use the code from Caddy, see [Caddy's docs](https://caddyserver.com/docs/startup).
* *kubernetes* now implements a `fallthrough` option to pass queries that would result in NXDOMAIN
  to the next plugin, even if the query is in the cluster domain. This enables custom DNS
  entries in the cluster domain (as long as they do not overlap with a normal Kubernetes record). To
  facilitate this the plugin ordering is also altered to put *kubernetes* in the chain before
  other backends.
* *cache* will no longer cache RRSIGs that will expire while cached.

# Contributors

The following people helped with getting this release done:

Chris Aniszczyk,
Chris O'Haver,
Christoph Görn,
Dominic,
John Belamaric,
Jonathan Boulle,
Michael,
Michael S. Fischer,
Miek Gieben,
Yong Tang,
Yue Ko.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
