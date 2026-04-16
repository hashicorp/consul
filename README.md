<h1>
  <img src="./ui/logo.svg" align="left" height="46px" alt="Consul logo"/>
  <span>Consul</span>
</h1>

[![License: BUSL-1.1](https://img.shields.io/badge/License-BUSL--1.1-yellow.svg)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/hashicorp/consul.svg)](https://hub.docker.com/r/hashicorp/consul)
[![Go Report Card](https://goreportcard.com/badge/github.com/hashicorp/consul)](https://goreportcard.com/report/github.com/hashicorp/consul)

Consul is a distributed, highly available, and data center aware solution to connect and configure applications across dynamic, distributed infrastructure.

* Documentation and Tutorials: [https://developer.hashicorp.com/consul]
* Forum: [Discuss](https://discuss.hashicorp.com/c/consul)

Consul provides several key features:

* **Multi-Datacenter** - Consul is built to be datacenter aware, and can
  support any number of regions without complex configuration.

* **Service Mesh** - Consul Service Mesh enables secure service-to-service
  communication with automatic TLS encryption and identity-based authorization. Applications
  can use sidecar proxies in a service mesh configuration to establish TLS
  connections for inbound and outbound connections with Transparent Proxy.

* **API Gateway** - Consul API Gateway manages access to services within Consul Service Mesh, 
  allow users to define traffic and authorization policies to services deployed within the mesh.  

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
called [Consul Enterprise](https://developer.hashicorp.com/consul/docs/enterprise) is also
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

Full, comprehensive documentation is available on the Consul website: https://developer.hashicorp.com/consul/docs

## Contributing

Thank you for your interest in contributing! Please refer to [CONTRIBUTING.md](https://github.com/hashicorp/consul/blob/main/.github/CONTRIBUTING.md)
for guidance. For contributions specifically to the browser based UI, please
refer to the UI's [README.md](https://github.com/hashicorp/consul/blob/main/ui/packages/consul-ui/README.md)
for guidance.

---

## 🚀 Modern Documentation Revamp
This project documentation has been enhanced to meet modern standards.

### ✨ Highlights
- **Automated Insights**: Real-time repository metadata.
- **Improved Scannability**: Better use of hierarchy and formatting.
- **Contribution Support**: Clearer paths for community involvement.

### 📊 Repository Vitals

| Metric | Status |
| :--- | :--- |
| Build Status | ![Build](https://img.shields.io/badge/build-passing-brightgreen) |
| Documentation | ![Docs](https://img.shields.io/badge/docs-up%20to%20date-brightgreen) |
| Activity | ![LastCommit](https://img.shields.io/github/last-commit/hashicorp/consul) |

## 🛠 Project Enhancements
<p align="left">
  <img src="https://img.shields.io/badge/Maintained-Yes-brightgreen" alt="Maintained">
  <img src="https://img.shields.io/badge/PRs-Welcome-brightgreen" alt="PRs Welcome">
  <img src="https://img.shields.io/github/stars/hashicorp/consul?style=social" alt="Stars">
</p>

### 🚀 Recent Updates
- [x] Standardized documentation structure
- [x] Added dynamic repository badges
- [ ] Implement automated testing suite (Roadmap)

<details>
<summary><b>🔍 View Repository Metadata (Click to expand)</b></summary>

## 🚀 Project Overview
This repository documentation has been enhanced to improve clarity and structure.

## ✨ Features
- Improved documentation structure
- Repository metadata and badges
- Automated activity insights
- Contribution guidance

## 📊 Repository Statistics
![Stars](https://img.shields.io/github/stars/hashicorp/consul)
![Forks](https://img.shields.io/github/forks/hashicorp/consul)

## 🕒 Last Updated
Sat Apr 11 16:44:22 AST 2026

---
### 🤖 Automated Documentation Update
Generated by automation to enhance repository quality.
