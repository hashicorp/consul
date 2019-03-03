+++
title = "CoreDNS-1.1.4 Release"
description = "CoreDNS-1.1.4 Release Notes."
tags = ["Release", "1.1.4", "Notes"]
release = "1.1.4"
date = "2018-06-19T09:39:29+01:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.1.4) of
CoreDNS-1.1.4!

This release has a few enhancements in the plugins, and a few (Docker) improvements.

# Core

As said in the [1.1.3 Release Notes](/2018/05/24/coredns-1.1.3-release/), we are making the `-log`
command line flag a noop.

This is also a heads up that in the next release - 1.2.0 - the current *etcd* plugin will be
replaced by a new plugin that supports etcd3, see this [pull
request](https://github.com/coredns/coredns/pull/1702).

The Docker image is now built using a multistage build. This means the final image is based on
`scratch` *and* all architectures now have certificates in the image (not just the amd64 one).

# Plugins

We are also deprecating:

* the *reverse* plugin has been removed, but we allow it still in the configuration.
* the `google_https` protocol has been a noop in the *proxy* plugin.

In the next release (1.2.0) this code will removed completely.

Further more:

* *file* now always queries local zones when trying to find a CNAME target.
* *log* will now always log in seconds (not micro, or milliseconds).
* *forward* erases expired connection after some time.

## Contributors

The following people helped with getting this release done:

Francois Tur,
Malcolm Akinje,
Mario Kleinsasser,
Miek Gieben,
Ruslan Drozhdzh,
Yong Tang.

For documentation see our (in progress!) [manual](/manual). For help and other resources, see our
[community page](https://coredns.io/community/).
