+++
title = "CoreDNS-1.6.3 Release"
description = "CoreDNS-1.6.3 Release Notes."
tags = ["Release", "1.6.3", "Notes"]
release = "1.6.3"
date = 2019-08-31T07:30:47+00:00
author = "coredns"
+++

The CoreDNS team has released
[CoreDNS-1.6.3](https://github.com/coredns/coredns/releases/tag/v1.6.3).

In this release we have moved the *federation* plugin to
[github.com/coredns/federation](https://github.com/coredns/federation), but it is still fully
functional in this version. In version 1.7.0 we expect to deprecate it.

Further more a slew a spelling corrections, and other minor improvements and polish. **And** two new
plugins: *clouddns* and *sign*.

# Plugins

* [*clouddns*](/plugins/clouddns) to enable serving zone data from GCP Cloud DNS.
* [*sign*](/plugins/sign) that (DNSSEC) signs your zonefiles (in its most basic form).
* [*file*](/plugins/file) various update, plug a memory leak when doing zone transfers, among other
  things.

We've removed the time stamping from `pkg/log` as timestamps are *also* added by the logging
aggregators, like `journald` or inside Kubernetes. And a small ASCII art logo is now printed when
CoreDNS starts up.

## Brought to You By

AllenZMC,
Chris Aniszczyk,
Chris O'Haver,
Cricket Liu,
Guangming Wang,
Julien Garcia Gonzalez,
li mengyang,
Miek Gieben,
Muhammad Falak R Wani,
Palash Nigam,
Sakura,
wwgfhf,
xieyanker,
Xigang Wang,
Yevgeny Pats,
Yong Tang,
zhangguoyan,
陈谭军.


## Noteworthy Changes

* fuzzing: Add Continuous Fuzzing Integration to Fuzzit (https://github.com/coredns/coredns/pull/3093)
* pkg/log: remove timestamp (https://github.com/coredns/coredns/pull/3218)
* plugin/clouddns: Add Google Cloud DNS plugin (https://github.com/coredns/coredns/pull/3011)
* plugin/federation: Move federation plugin to github.com/coredns/federation (https://github.com/coredns/coredns/pull/3139)
* plugin/file: close reader for reload (https://github.com/coredns/coredns/pull/3196)
* plugin/file: less notify logging spam (https://github.com/coredns/coredns/pull/3212)
* plugin/file: respond correctly to IXFR message (https://github.com/coredns/coredns/pull/3177)
* plugin/file: rework outgoing axfr (https://github.com/coredns/coredns/pull/3227)
* plugin/{health,ready}: return standardized text for ready and health endpoint (https://github.com/coredns/coredns/pull/3195)
* plugin/k8s_external handle NS records (https://github.com/coredns/coredns/pull/3160)
* plugin/kubernetes: handle NS records (https://github.com/coredns/coredns/pull/3160)
* startup: add logo (https://github.com/coredns/coredns/pull/3230)
