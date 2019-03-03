+++
title = "CoreDNS-1.2.1 Release"
description = "CoreDNS-1.2.1 Release Notes."
tags = ["Release", "1.2.1", "Notes"]
release = "1.2.1"
date = "2018-08-28T07:10:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.2.1) of
CoreDNS-1.2.1!

This release features bugfixes (mostly in the [*kubernetes*](/plugins/kubernetes) plugin),
documentation improvements and one new plugin: [*loop*](/plugins/loop).

# Plugins

* A new plugin called [*loop*](/plugins/loop) was added. When starting up it detects resolver loops
  and stops the process if one is detected.

## Contributors

The following people helped with getting this release done. Good to see a whole bunch of new names,
as well as the usual suspects:

Bingshen Wang,
Chris O'Haver,
Eugen Kleiner,
Francois Tur,
Jiacheng Xu,
Karsten Weiss,
Lorenzo Fontana,
Miek Gieben,
Nitish Tiwari,
Stanislav Zapolsky,
varyoo,
Yong Tang,
Zach Eddy.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
