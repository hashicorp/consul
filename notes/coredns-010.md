+++
title = "CoreDNS-010 Release"
description = "CoreDNS-010 Release Notes."
tags = ["Release", "010", "Notes"]
draft = false
release = "010"
date = "2017-07-25T11:24:43-04:00"
author = "coredns"
+++

CoreDNS-010 has been [released](https://github.com/coredns/coredns/releases/tag/v010)!

CoreDNS is a DNS server that chains plugins, where each plugin implements a DNS feature.

Release v010 is mostly a bugfix release, with one new plugin - *dnstap*.

# Core

No changes.

# Plugins

## New

* *dnstap* is a new plugin that allows you to get dnstap information from CoreDNS.

## Updates

* *file* now handles multiple wildcard below each other correctly, and handles wildcards at the apex.
* *hosts*, and *kubernetes* have been fixed to return success with no data in cases where records exist
but not of the requested type. This fixes an issue with getting NXDOMAIN for the AAAA record even when the
A record exists confusing some resolvers.

# Documentation

* Many updates to README files.

# Contributors

The following people helped with getting this release done:

Antoine Debuisson,
Chris O'Haver,
John Belamaric,
Miek Gieben,
Pat Moroney

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
