+++
date = "2017-01-01T10:30:31Z"
description = "CoreDNS-004 Release Notes."
release = "004"
tags = ["Release", "004", "Notes"]
title = "CoreDNS-004 Release"
author = "coredns"
+++

CoreDNS-004 has been [released](https://github.com/coredns/coredns/releases/tag/v004)!

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

# What is New

## Core

We are now also releasing an ARM build that can run on Raspberry Pi.

## Plugin improvements

* *file|auto*: resolve external CNAME when an upstream (new option) is specified.
* *file|auto*: allow port numbers for transfer from/to to be specified.
* *file|auto*: include zone's NSset in positive responses.
* *auto*: close files and don't leak file descriptors.
* *httpproxy*: new plugin that proxies to <https://dns.google.com> and resolves your requests over an encrypted connection. This plugin will probably be morphed into proxy at some point in the feature. Consider it experimental for the time being.
* *metrics*: `response_size_bytes` and `request_size_bytes` export the actual length of the packet, not the advertised bufsize.
* *log*: `{size}` is now the length in bytes of the request, `{rsize}` for the reply. Default logging is changed to show both.

# Contributors

The following people helped with getting this release done:

Chris O'Haver,
Dmytro Kislov,
John Belamaric,
Mark Nevill,
Michael Grosser,
Miek Gieben,
Yong Tang.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
