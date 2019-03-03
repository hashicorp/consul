+++
title = "CoreDNS-1.0.0 Release"
description = "CoreDNS-1.0.0 Release Notes."
tags = ["Release", "1.0.0", "Notes"]
draft = false
release = "1.0.0"
date = "2017-12-01T22:43:43-00:00"
author = "coredns"
+++

We are pleased to announce the [release](https://github.com/coredns/coredns/releases/tag/v1.0.0) of CoreDNS-1.0.0!

Release 1.0.0 and other recent releases have focused on improving the performance and
functionality of the *kubernetes* plugin, since CoreDNS is now on track to eventually
[replace kube-dns](https://github.com/kubernetes/features/issues/427) as the default
cluster DNS in Kubernetes.

As part of the [Kubernetes proposal](https://github.com/kubernetes/community/pull/1100), we have shown that CoreDNS
not only provides more functionality than kube-dns, but performs much better while using less memory. In our tests,
[CoreDNS](https://github.com/kubernetes/community/pull/1100#issuecomment-337747482) running against a cluster with 5000
services was able to process 18,000 queries per second using 73MB of RAM, while
[kube-dns](https://github.com/kubernetes/community/pull/1100#issuecomment-338329100) achieved 7,000qps using 97MB of RAM.
This can be partial ascribed to CoreDNS simpler runtime - a single process instead of a combination of several processes.

CoreDNS also implements a number of Kubernetes-related features that are not part of kube-dns, including:

* Filtering of records by namespace
* Filtering of records by label selector
* `pods verified` mode, which ensures that a Pod exists before returning an answer for a `pod.cluster.local` query
* `endpoint_pod_names` which uses [Pod names](https://github.com/kubernetes/kubernetes/issues/47992) for service endpoint records if the hostname is not set
* `autopath` which provides a server-side implementation of the namespace-specific search path. This can cut down the query latency from pods dramatically.

As a general-purpose DNS server, CoreDNS also enables many other use cases that would be difficult or impossible to
achieve with kube-dns, such as the ability to create [custom DNS entries](https://coredns.io/2017/05/08/custom-dns-entries-for-kubernetes/).

We are excited to continue our contributions to the Kubernetes community, and CoreDNS is being incorporated as a 1.9 alpha feature into a variety
of Kubernetes deployment mechanisms, including upcoming versions of [kubeadm](https://github.com/kubernetes/kubeadm), [kops](https://github.com/kubernetes/kops), [minikube](https://github.com/kubernetes/minikube), and [kubespray](https://github.com/kubernetes-incubator/kubespray).

Of course, there is more to 1.0.0 than just the Kubernetes work. See below for the details on all the changes.

## Core

* Fixed a bug in the gRPC server that prevented *dnstap* from working with it.
* Additional fuzz testing to ferret out obscure bugs.
* Documentation and configuration cleanups.

## Plugins
* *log* no longer accepts `stdout` in the configuration (use of a file was removed in a previous release). All logging is always to STDOUT. This is a backwards **incompatible** change, so be sure to check your Corefile for this.
* *health* now checks plugins that support it for health and reflects that in the server health.
* *kubernetes* now shows healthy only after the initial API sync is complete.
* *kubernetes* has bug fixes and performance improvements.
* *kubernetes* now has an option to use pod names instead of IPs in service endpoint records when the `hostname` is not set.
* *metrics* have been revised to provide better histograms. You will need to change your Prometheus queries as metric names have changed to comply with Prometheus best practices.
* *erratic* now supports the health check.

## Contributors

The following people helped with getting this release done:
Andy Goldstein,
Ben Kochie,
Brian Akins,
Chris O'Haver,
Christian Nilsson,
John Belamaric,
Max Schmitt,
Michael Grosser,
Miek Gieben,
Ruslan Drozhdzh,
Uladzimir Trehubenka,
Yong Tang.

If you want to help, please check out one of the [issues](https://github.com/coredns/coredns/issues/)
and start coding!

For documentation and help, see our [community page](https://coredns.io/community/).
