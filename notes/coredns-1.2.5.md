+++
title = "CoreDNS-1.2.5 Release"
description = "CoreDNS-1.2.5 Release Notes."
tags = ["Release", "1.2.5", "Notes"]
release = "1.2.5"
date = "2018-10-24T20:40:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.2.5) of
CoreDNS-1.2.5!

## Core

Correctly make a reply fit in the client's buffer, *especially* when EDNS0 is not used.
This used to be the responsibility of a plugin, now the server will handle it.

## Plugins

Documentation and smaller updates for various plugins, as well as:

* [*cache*](/plugins/cache) - resets min TTL default back to 5 second (instead of 0).
* [*dnssec*](/plugins/dnssec) - now allows aZSK/KSK split as well as a CSK setup.
* [*rewrite*](/plugins/rewrite) - answer rewrite is now automatic for _exact_ name rewrites.

## Brought to you by

Andrey Meshkov,
Chris O'Haver,
Francois Tur,
Kevin Nisbet,
Manuel Stocker,
Miek Gieben,
Paul G,
Ruslan Drozhdzh,
Sandeep Rajan,
Yong Tang.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
