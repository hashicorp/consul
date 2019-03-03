+++
title = "CoreDNS-1.0.3 Release"
description = "CoreDNS-1.0.3 Release Notes."
tags = ["Release", "1.0.3", "Notes"]
draft = false
release = "1.0.3"
date = "2018-01-10T19:38:29+00:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.0.3) of CoreDNS-1.0.3!
This is a small bugfix release, but we also have a new plugin:
[*template*](https://coredns.io/plugins/template).

## Core

Manual pages are now generated from the READMEs, you can find them in the man/ directory.
A coredns(1) and corefile(5) one where also added.

## Plugins

The `fallthrough` directive was overhauled and now allows a list of zones to be specified. It will
then only fallthrough for those zones, see `plugin/plugin.md`.

A new plugin *template* was added. It allows you to use Go (text) templates to craft a response, see
<https://coredns.io/plugins/template> for docs.

* *dnssec* implements Cloudflares's NSEC blacklies better.
* *kubernetes*, adds a fix for `pod insecure` look ups for non-IP addresses.
* *health* adds a metrics for the duration it takes to GET /health. Useful for getting a sense of
  overloadedness of the process.

## Contributors

The following people helped with getting this release done:
John Belamaric,
Miek Gieben,
Rene Treffer,
Yong Tang.

If you want to help, please check out one of the
[issues](https://github.com/coredns/coredns/issues/) and start coding! For documentation and help,
see our [community page](https://coredns.io/community/).
