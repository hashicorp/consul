# Consul [![CircleCI](https://circleci.com/gh/hashicorp/consul/tree/master.svg?style=svg)](https://circleci.com/gh/hashicorp/consul/tree/master) [![Discuss](https://img.shields.io/badge/discuss-consul-ca2171.svg?style=flat)](https://discuss.hashicorp.com/c/consul)

* Website: https://www.consul.io
* Forum: [Discuss](https://discuss.hashicorp.com/c/consul)

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

* **Service Segmentation/Service Mesh** - Consul Connect enables secure service-to-service
  communication with automatic TLS encryption and identity-based authorization. Applications 
  can use sidecar proxies in a service mesh configuration to automatically establish TLS 
  connections for inbound and outbound connections without being aware of Connect at all. 

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
