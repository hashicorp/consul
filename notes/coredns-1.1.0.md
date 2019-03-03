+++
title = "CoreDNS-1.1.0 Release"
description = "CoreDNS-1.1.0 Release Notes."
tags = ["Release", "1.1.0", "Notes"]
draft = false
release = "1.1.0"
date = "2018-03-12T09:33:29+00:00"
author = "coredns"
enabled = "default"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.1.0) of
CoreDNS-1.1.0!

CoreDNS has been promoted to the [incubating](https://www.cncf.io/projects/graduation-criteria/)
level in the [CNCF](https://www.cncf.io/projects/)!
This has been made possible by the work done by contributors, users and adopters.

**Thank you all!**

## Core

Bump the version to 1.1.0, as we deprecate two plugins (*shutdown* and *startup*).

In CoreDNS 1.0.6 the [*bind*](/plugins/bind) plugin was extended to allow binding to multiple
interfaces. This release adds the ability serve the same zone on different interfaces (we used to
block this for no good reason). I.e. this now works:

```
. {
    bind 127.0.0.1
    # ..
}

. {
    bind 127.0.0.2
    # ...
}
```

## Plugins

* The plugins *shutdown* and *startup* where marked deprecated in 1.0.6. This release removes them. You should use [*on*](/explugins/on) instead.
* A new plugin was added: *reload*, which watches for changes in your Corefile and then automatically will reload the process. This is not yet bullet proof, some plugins can fail to setup during a reload. See the discussion in [issue 1445](https://github.com/coredns/coredns/issues/1455).
* A number of plugins can only be used once in a server block, but didn't make this explicit. I.e. [*dnssec*](/plugins/dnssec) would silently overwrite earlier config. The following plugins now return an error when used multiple times **in a single Server Block**:
*cache*,
*dnssec*,
*errors*,
*forward*,
*hosts*,
*nsid*,
*metrics*,
*kubernetes*,
*pprof*,
*reload*,
*root*.
* [*Trace*](/plugins/trace) adds support for a Datadog endpoint.
* Some changes went into [*dnstap*](/plugins/dnstap), make it easier to use from other plugins.
* Small change in the [*log*](/plugin/log) plugin, the log default will now also log the client's
  port number and IPv6 addresses are printed with brackets: `[::1]`.

## Contributors

The following people helped with getting this release done:

Chris O'Haver,
Francois Tur,
John Belamaric,
Miek Gieben,
nogoegst,
Ricardo Katz,
Tobias Schmidt,
Uladzimir Trehubenka,
varyoo,
Yamil Asusta,
Yong Tang.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
