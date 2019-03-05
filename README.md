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

**Please note**: We take Consul's security and our users' trust very seriously. If you 
believe you have found a security issue in Consul, please [responsibly disclose](https://www.hashicorp.com/security#vulnerability-reporting) by 
contacting us at security@hashicorp.com.

## Quick Start

An extensive quick start is viewable on the Consul website:

https://www.consul.io/intro/getting-started/install.html

## Documentation

Full, comprehensive documentation is viewable on the Consul website:

https://www.consul.io/docs

## Contributing

Thank you for your interest in contributing! Please refer to [CONTRIBUTING.md](https://github.com/hashicorp/consul/blob/master/.github/CONTRIBUTING.md) for guidance.