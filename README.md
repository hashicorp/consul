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

Consul runs on Linux, Mac OS X, FreeBSD, Solaris, and Windows.

## Quick Start

An extensive quick start is viewable on the Consul website:

https://www.consul.io/intro/getting-started/install.html

## Documentation

Full, comprehensive documentation is viewable on the Consul website:

https://www.consul.io/docs

## Developing Consul

To develop Consul locally, you will need:

- [Golang](https://golang.org)
- [Docker for Mac](https://docs.docker.com/docker-for-mac/), [Docker for Windows](https://docs.docker.com/docker-for-windows/), or [Docker for Linux](https://docs.docker.com/engine/installation/)
- GNU-compatible make
- Bash (for some operations)

Clone this repository into `$GOPATH/src/github.com/hashicorp/consul` and
run `make bootstrap`. This will install any dependencies.

```shell
$ make bootstrap
```

To build a local binary of Consul, run `make dev`

```shell
$ make dev
```

To build all binaries, run `make bin`

```shell
$ make bin
```

To build the UI, run `make ui`

```shell
$ make ui
```

To run the tests, run `make test`

```shell
$ make test
```

If you make any changes to the code, run `make format` in order to automatically
format the code according to Go standards.

## Vendoring

Consul currently uses [govendor](https://github.com/kardianos/govendor) for
vendoring.
