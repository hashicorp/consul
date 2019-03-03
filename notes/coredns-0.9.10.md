+++
title = "CoreDNS-0.9.10 Release"
description = "CoreDNS-0.9.10 Release Notes."
tags = ["Release", "0.9.10", "Notes"]
draft = false
release = "0.9.10"
date = "2017-11-03T20:45:43-00:00"
author = "coredns"
+++

CoreDNS-0.9.10 has been [released](https://github.com/coredns/coredns/releases/tag/v0.9.10)!

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

Release 0.9.10 is a minor release, with some fixes.

## Core

* The reverse zone syntax was extended to allow non-octet boundaries:

   ~~~
   192.168.1.0/17 {
       ...
   }
   ~~~

   Will behave correctly.

* Lots of documentation clean ups.
* More platforms have binaries for each release.

## Plugins

* *dnssec* will now insert DS records (and sign them) when it signs a delegation response.
* *host* now checks for /etc/hosts updates in a separate go-routine.

## Contributors

The following people helped with getting this release done:
Chris O'Haver,
Miek Gieben,
Pat Moroney,
Paul Hoffman,
Sandeep Rajan,
Yong Tang.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
