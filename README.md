<h1>
  <img src="./website/public/img/logo.svg" align="left" height="46px" alt="Consul logo"/>
  <span>Consul</span>
</h1>

[![License: BUSL-1.1](https://img.shields.io/badge/License-BUSL--1.1-yellow.svg)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/hashicorp/consul.svg)](https://hub.docker.com/r/hashicorp/consul)
[![Go Report Card](https://goreportcard.com/badge/github.com/hashicorp/consul)](https://goreportcard.com/report/github.com/hashicorp/consul)

Consul is a distributed, highly available, and data center aware solution to connect and configure applications across dynamic, distributed infrastructure.

* Website: https://www.consul.io
* Tutorials: [HashiCorp Learn](https://learn.hashicorp.com/consul)
* Forum: [Discuss](https://discuss.hashicorp.com/c/consul)

Consul provides several key features:

* **Multi-Datacenter** - Consul is built to be datacenter aware, and can
  support any number of regions without complex configuration.

* **Service Mesh** - Consul Service Mesh enables secure service-to-service
  communication with automatic TLS encryption and identity-based authorization. Applications
  can use sidecar proxies in a service mesh configuration to establish TLS
  connections for inbound and outbound connections with Transparent Proxy.

* **Service Discovery** - Consul makes it simple for services to register
  themselves and to discover other services via a DNS or HTTP interface.
  External services such as SaaS providers can be registered as well.

* **Health Checking** - Health Checking enables Consul to quickly alert
  operators about any issues in a cluster. The integration with service
  discovery prevents routing traffic to unhealthy hosts and enables service
  level circuit breakers.

* **Dynamic App Configuration** - An HTTP API that allows users to store indexed objects within Consul,
   for storing configuration parameters and application metadata.

Consul runs on Linux, macOS, FreeBSD, Solaris, and Windows and includes an
optional [browser based UI](https://demo.consul.io). A commercial version
called [Consul Enterprise](https://www.consul.io/docs/enterprise) is also
available.

**Please note**: We take Consul's security and our users' trust very seriously. If you
believe you have found a security issue in Consul, please [responsibly disclose](https://www.hashicorp.com/security#vulnerability-reporting)
by contacting us at security@hashicorp.com.

## Quick Start

A few quick start guides are available on the Consul website:

* **Standalone binary install:** https://learn.hashicorp.com/collections/consul/get-started-vms
* **Minikube install:** https://learn.hashicorp.com/tutorials/consul/kubernetes-minikube
* **Kind install:** https://learn.hashicorp.com/tutorials/consul/kubernetes-kind
* **Kubernetes install:** https://learn.hashicorp.com/tutorials/consul/kubernetes-deployment-guide
* **Deploy HCP Consul:** https://learn.hashicorp.com/tutorials/consul/hcp-gs-deploy 

## Documentation

Full, comprehensive documentation is available on the Consul website: https://consul.io/docs

## Contributing

Thank you for your interest in contributing! Please refer to [CONTRIBUTING.md](https://github.com/hashicorp/consul/blob/main/.github/CONTRIBUTING.md)
for guidance. For contributions specifically to the browser based UI, please
refer to the UI's [README.md](https://github.com/hashicorp/consul/blob/main/ui/packages/consul-ui/README.md)
for guidance.
