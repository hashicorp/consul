# Consul [![Build Status](https://travis-ci.org/hashicorp/consul.svg?branch=master)](https://travis-ci.org/hashicorp/consul) [![Join the chat at https://gitter.im/hashicorp-consul/Lobby](https://badges.gitter.im/hashicorp-consul/Lobby.svg)](https://gitter.im/hashicorp-consul/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

* Website: https://www.consul.io
* Chat: [Gitter](https://gitter.im/hashicorp-consul/Lobby)
* Mailing list: [Google Groups](https://groups.google.com/group/consul-tool/)

Consul is a tool for service discovery and configuration. Consul is
distributed, highly available, and extremely scalable.

Consul provides several key features:

* **Service Discovery** - Consul makes it simple for services to register
  themselves and to discover other services via a DNS or HTTP interface.
  External services such as SaaS providers can be registered as well.

* **Health Checking** - Health Checking enables Consul to quickly alert
  operators about any issues in a cluster. The integration with service
  discovery prevents routing traffic to unhealthy hosts and enables service
  level circuit breakers.

* **Key/Value Storage** - A flexible key/value store enables storing
  dynamic configuration, feature flagging, coordination, leader election and
  more. The simple HTTP API makes it easy to use anywhere.

* **Multi-Datacenter** - Consul is built to be datacenter aware, and can
  support any number of regions without complex configuration.

* **Service Segmentation** - Consul Connect enables secure service-to-service 
communication with automatic TLS encryption and identity-based authorization.

Consul runs on Linux, Mac OS X, FreeBSD, Solaris, and Windows. A commercial
version called [Consul Enterprise](https://www.hashicorp.com/products/consul)
is also available.

## Quick Start

An extensive quick start is viewable on the Consul website:

https://www.consul.io/intro/getting-started/install.html

## Documentation

Full, comprehensive documentation is viewable on the Consul website:

https://www.consul.io/docs

## Developing Consul

If you wish to work on Consul itself, you'll first need [Go](https://golang.org)
installed (version 1.9+ is _required_). Make sure you have Go properly installed,
including setting up your [GOPATH](https://golang.org/doc/code.html#GOPATH).

Next, clone this repository into `$GOPATH/src/github.com/hashicorp/consul` and
then just type `make`. In a few moments, you'll have a working `consul` executable:

```
$ make
...
$ bin/consul
...
```

*Note: `make` will build all os/architecture combinations. Set the environment variable `CONSUL_DEV=1` to build it just for your local machine's os/architecture, or use `make dev`.*

*Note: `make` will also place a copy of the binary in the first part of your `$GOPATH`.*

You can run tests by typing `make test`. The test suite may fail if
over-parallelized, so if you are seeing stochastic failures try
`GOTEST_FLAGS="-p 2 -parallel 2" make test`.

If you make any changes to the code, run `make format` in order to automatically
format the code according to Go standards.

## Vendoring

Consul currently uses [govendor](https://github.com/kardianos/govendor) for
vendoring and [vendorfmt](https://github.com/magiconair/vendorfmt) for formatting
`vendor.json` to a more merge-friendly "one line per package" format.
