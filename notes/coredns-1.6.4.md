+++
title = "CoreDNS-1.6.4 Release"
description = "CoreDNS-1.6.4 Release Notes."
tags = ["Release", "1.6.4", "Notes"]
release = "1.6.4"
date = 2019-09-27T10:00:00+00:00
author = "coredns"
+++

The CoreDNS team has released
[CoreDNS-1.6.4](https://github.com/coredns/coredns/releases/tag/v1.6.4).

Various code cleanups and documentation improvements. We've added one new plugin: *acl*, that allows
blocking requests.

# Plugins

* [*acl*](/plugins/acl) block request from IPs or IP ranges.
* [*kubernetes*](/plugins/kubernetes) received some bug fixes, see below for specific PRs.
* [*hosts*](/plugins/hosts) exports metrics on the number of entries and last reload time.

## Brought to You By

An Xiao,
Chris O'Haver,
Cricket Liu,
Guangming Wang,
Kasisnu,
li mengyang,
Miek Gieben,
orangelynx,
xieyanker,
yeya24,
Yong Tang.

## Noteworthy Changes

* plugin/hosts: add host metrics (https://github.com/coredns/coredns/pull/3277)
* plugin/kubernetes: Don't duplicate service record for every port (https://github.com/coredns/coredns/pull/3240)
* plugin/kubernetes: Handle multiple local IPs and bind (https://github.com/coredns/coredns/pull/3208)
* Add plugin ACL for source IP filtering (https://github.com/coredns/coredns/pull/3103)
