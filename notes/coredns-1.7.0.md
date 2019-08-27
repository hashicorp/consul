+++
title = "CoreDNS-1.7.0 Release"
description = "CoreDNS-1.7.0 Release Notes."
tags = ["Release", "1.7.0", "Notes"]
release = "1.7.0"
date = 2019-08-31T14:35:47+01:00
author = "coredns"
+++

The CoreDNS team has released
[CoreDNS-1.7.0](https://github.com/coredns/coredns/releases/tag/v1.7.0).

In this release we have deprecated the *federation* plugin that was used in conjunction with the
*kubernetes* plugin.

Further more a slew a spelling corrections and other minor improvements and polish. **And** three(!)
new plugins.

# Plugins

* [*acl*](/plugins/acl) blocks queries depending on their source IP address.
* [*clouddns*](/plugin/clouddns) to enable serving zone data from GCP Cloud DNS.
* [*sign*](/plugins/sign) that (DNSSEC) signs your zonefiles (in its most basic form).

## Brought to You By


## Noteworthy Changes

* plugin/clouddns: Add Google Cloud DNS plugin (https://github.com/coredns/coredns/pull/3011)
* plugin/federation: Move federation plugin to github.com/coredns/federation (https://github.com/coredns/coredns/pull/3139)
* plugin/file: close reader for reload (https://github.com/coredns/coredns/pull/3196)
* plugin/file: respond correctly to IXFR message (https://github.com/coredns/coredns/pull/3177)
* plugin/k8s_external handle NS records (https://github.com/coredns/coredns/pull/3160)
* plugin/kubernetes: handle NS records (https://github.com/coredns/coredns/pull/3160)
