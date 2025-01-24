# Consul Documentation | Information Architecture and Content Strategy

This sub-directory contains the written content that appears in [the Consul documentation developer.hashicorp.com](https://developer.hashicorp.com/consul).

## Content overview

The `website/content` directory in the `hashicorp/consul` repo contains the following sub-directories:

```
.
├── api-docs
├── commands
├── docs
└── partials
```

After you merge a PR into a numbered release branch, changes to these folders appear at the following URLs:

- Changes to `api-docs` appear at [https://developer.hashicorp.com/consul/api-docs](https://developer.hashicorp.com/consul/api-docs).
- Changes to `commands` appear at [https://developer.hashicorp.com/consul/commands](https://developer.hashicorp.com/consul/commands)
- Changes to `docs` appear at [https://developer.hashicorp.com/consul/docs](https://developer.hashicorp.com/consul/docs)

URLs follow the directory structure for each folder and omit the the `.mdx` file extension. Pages named `index.mdx` adopt their directory's name. For example, the file `docs/reference/agent/index.mdx` appears at the URL [https://developer.hashicorp.com/consul/docs/reference/agent](https://developer.hashicorp.com/consul/docs/reference/agent).

The `partials` folder contains content that you can reuse across pages in any of the three folders. Refer to [Guide to Partials](#guide-to-partials) for more information.

Tutorials that appear at [https://developer.hashicorp.com/consul/tutorials](https://developer.hashicorp.com/consul/tutorials) are located in a different repository. This content exists in the [hashicorp/tutorials GitHub repo](https://github.com/hashicorp/tutorials), which is internal to the HashiCorp organization.

## North Star principles

The design of the content in the `docs/` directory, including structure, filepaths, and labelling, is governed by the following _north star principles_.

1. **Users are humans**. Design for humans first. For example, file paths become URLs; create human-readable descriptions of the content and avoid unnecessary repetition.
1. **Less is always more**. Prefer single words for folder and file names; add a hyphen and a second word to disambiguate from existing content.
1. **Document what currently exists**. Do not create speculative folders and files to "reserve space" for future updates and releases. Do not describe Consul as it will exist in the future; describe it as it exists right now.
1. **Beauty works better**. When creating new files and directories, strive for consistency with the existing structure. For example, use parallel structures across directories and flatten directories that run unsually deep. Tip: If it doesn't look right, it's probably not right.
1. **Prefer partials over `ctrl+v`**. Spread content out, but document unique information in one place. When you need to repeat content across multiple pages, use partials to maintain content.

These principles exist to help you navigate ambiguity when making changes to the underlying content. If you add new content and you're not quite sure where to place it or how to name it, use these "north stars" to help you make an informed decision about where to go next.

## Taxonomy

There are three main categories in our docs. This division of categories is not literal to the directory structure, even though the **Reference** category includes the repo's `reference` folder.

- Intro
- Usage
- Reference

The categories align with Diataxis.

### Intro

The **Intro** category includes the following folders:

- `architecture`
- `concept`
- `enterprise`
- `fundamentals`
- `use-case`

The following table lists each term and a definition to help you decide where to place new content.

| Term         | Directory      | What it includes                                                                                                                                                 |
| :----------- | :------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Architecture | `architecture` | The product's components and their “maps” in cloud networking contexts.                                                                                          |
| Concept      | `concept`      | Describes the complex behavior of technical systems in a non-literal manner. For example, computers do not literally “gossip” when they use the gossip protocol. |
| Enterprise   | `enterprise`   | Consul Enterprise license offerings and how to implement them.                                                                                                   |
| Fundamentals | `fundamentals` | The knowledge, connection and authorization methods, interactions, configurations, and operations most users require to use the product.                         |
| Use case     | `use-case`     | The highest level goals practitioners have; a product function that “solves” enterprise concerns and usually competes with other products.                       |

### Usage

The **Usage** category includes the following folders:

- `automate`
- `connect`
- `deploy`
- `discover`
- `east-west`
- `envoy-extension`
- `integrate`
- `manage`
- `manage-traffic`
- `monitor`
- `multi-tenant`
- `north-south`
- `observe`
- `register`
- `release-notes`
- `secure`
- `secure-mesh`
- `upgrade`

These folders are organized into two groups that are not literal to the directory structure.

- **Operations**. User actions, workflows, and goals related to installing and operating Consul as a long-running daemon on multiple nodes in a network.
- **Service networking**. User actions, workflows, and goals related to implementing networking solutions for application workloads.

Each folder is named after a corresponding _phase_, which have a set order in the group.

#### Operations

Operations consists of the following phases, intentionally ordered to reflect the full lifecycle of a Consul agent.

| Phase                | Directory       | Description                                                                                                                                               |
| :------------------- | :-------------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Deploy Consul        | `deploy`        | The processes to install and start Consul server agents, client agents and dataplanes.                                                                    |
| Secure Consul        | `secure`        | The processes to set up and maintain secure communications with Consul agents, including ACLs, TLS, and gossip.                                           |
| Manage multi-tenancy | `multi-tenant`  | The processes to use one Consul datacenter for multiple tenants, including admin partitions, namespaces, network segments, and sameness groups.           |
| Manage Consul        | `manage`        | The processes to manage and customize Consul's behavior, including DNS forwarding on nodes, server disaster recovery, rate limiting, and scaling options. |
| Monitor Consul       | `monitor`       | The processes to export Consul logs and telemetry for insight into agent behavior.                                                                        |
| Upgrade Consul       | `upgrade`       | The processes to update the Consul version running in datacenters.                                                                                        |
| Release Notes        | `release-notes` | Describes new, changed, and deprecated features for each release of Consul and major associated binaries.                                                 |

#### Service networking

Service Networking consists of the following phases, intentionally ordered to reflect a recommended “order of operations.” Although these phases do not need to be complated in order, they provide an overall path for increased complexity in Consul’s service networking capabilities as you develop your network.

| Phase                      | Directory        | Description                                                                                                                              |
| :------------------------- | :--------------- | :--------------------------------------------------------------------------------------------------------------------------------------- |
| Register services          | `register`       | How to define services and health checks and then register them with Consul.                                                             |
| Discover services          | `discover`       | How to use Consul's service discovery features, including Consul DNS, service lookups, load balancing.                                   |
| Connect service mesh       | `connect`        | How to set up and use sidecar proxies in a Consul service mesh.                                                                          |
| Secure network north/south | `north-south`    | How to configure, deploy, and use the Consul API gateway to secure network ingress.                                                      |
| Link network east/west     | `east-west`      | How to connect Consul datacenters across regions, runtimes, and providers with WAN federation and cluster peering.                       |
| Secure mesh traffic        | `secure-mesh`    | How to secure service-to-service communication with service intentions and TLS certificates.                                             |
| Manage service traffic     | `manage-traffic` | How to route traffic between services in a service mesh, including service failover and progressive rollouts.                            |
| Observe service mesh       | `observe`        | How to observe service mesh telemetry and application performance, including Grafana.                                                    |
| Automate applications      | `automate`       | How to automate Consul and applications to update dynamically, including the KV store, Consul-Terraform-Sync (CTS), and Consul template. |

### Reference

The **Reference** category includes the following folders:

- `error-messages`
- `reference`
- `reference-cli`
- `troubleshoot`

The following table lists each term and a definition to help you decide where to place new content.

| Term           | Directory         | What it includes                                                                                                             |
| :------------- | :---------------- | :--------------------------------------------------------------------------------------------------------------------------- |
| Error Messages | `error-messsages` | Error messages and their causes, organized by runtime and Consul release binary.                                             |
| Reference      | `reference`       | All reference information for configuring Consul, its components, and the infrastructure it runs on.                         |
| CLI reference  | `reference-cli`   | Reference information for CLIs in associated binaries such as `consul-k8s`, `consul-dataplane`, and `consul-terraform-sync`. |
| Troubleshoot   | `troubleshoot`    | Instructions and guidance about how to figure out what's wrong with a Consul deployment.                                     |

## Path syntax

A major advantage of the information architecture described in this document is the filepath structure. This structure "tags" documentation with keywords that describe the page's content in order to optimize the documentation for Google SEO while also helping human users build a "mental model" of Consul.

Our syntax creates human-readable filepaths using a controlled vocabulary and intentional permutations. In usage docs, filepaths follow a repeating pattern of "Verb - Noun - Adjective" to describe the content and user goal.

For Consul operations, filepaths tend to have the following order:

```plaintext hideClipboard
Operation -> Component -> Runtime -> Action -> Provider
```

Examples:

- `deploy/server/k8s/platform/openshift` contains instructions on deploying a Consul server agent when running OpenShift.
- `secure/encryption/tls/rotate/vm` contains instructions for rotating TLS certificates Consul agents use to secure their communications when running on virtual machines.
- `upgrade/k8s/compatibility` contains information about compatible software versions to help you upgrade the version of Consul running on Kubernetes.

For service networking, filepaths tend to have the following order:

```plaintext hideClipboard
Phase -> Feature -> Action -> Runtime -> Interface
```

Examples:

- `discover/load-balancer/nginx` contains instructions for using NGINX as a load balancer based on Consul service discovery results.
- `east-west/cluster-peering/establish/k8s` contains instructions for creating new connections between Consul clusters running on Kubernetes in different regions or cloud providers.

## Controlled vocabulary

Consul's product-specific vocabulary is sub-divided into the following categories:

- `architecture`: The product's components and their “maps” in cloud networking contexts.
- `concepts`: Describes the complex behavior of technical systems in a non-literal manner.
- `config-entry`: The configuration entries used to configure application behavior in the service mesh.
- `envoy-extensions`: The third-party Envoy plugins that Consul supports.
- `enterprise`: Features and operations available with a Consul Enterprise license.
- `features`: Core components that facilitate the operation of Consul's unique offerings.
- `fundamentals`: The knowledge, connection and authorization methods, interactions, configurations, and operations most users require to use the product.
- `use-cases`: The highest level goals practitioners have; a product function that “solves” enterprise concerns and usually competes with other products.

The following tables list the controlled vocabulary associated with each category.

### Architecture vocabulary

Consul's architecture vocabulary is structured according to where components run:

- `control-plane`: The _control plane_ is the network infrastructure that maintains a central registry to track services and their respective IP addresses. Both server and client agents operate as part of the control plane. Consul dataplanes, despite the name, are also part of the Consul control plane.
- `data-plane`: Use two words, _data plane_, to refer to the application layer and components involved in service-to-service communication.

Common architecture terms and where they run:

| Control plane  | Data plane |
| :------------- | :--------- |
| `agent`        | `gateway`  |
| `server agent` | `mesh`     |
| `client agent` | `proxy`    |
| `dataplane`    | `service`  |

The **Reference** category also includes an `architecture` sub-directory. This "Reference architecture" includes information such as port requirements, server requirements, and AWS ECS architecture.

### Concepts vocabulary

Consul's concept vocabulary collects terms that describe how internal systems operate through human actions.

- `catalog`: Covers Consul's running record of the services it registered, including addresses and health check results.
- `consensus`: Covers the server agent elections governed by the Raft protocol.
- `consistency`: Covers Consul's anti-entropy features, consistency modes, and Jepsen testing.
- `gossip`: Covers Serf communication between Consul agents in a datacenter.
- `reliability`: Cover fault tolerance, quorum size, and server redundancy.

### Configuration entry vocabulary

Consul's configuration entry vocabulary collects terms that describe

- `api-gateway`
- `control-plane-request-limit`
- `exported-services`
- `file-system-certificate`
- `http-route`
- `ingress-gateway`
- `inline-certificate`
- `jwt-provider`
- `mesh`
- `proxy-defaults`
- `sameness-group`
- `service-defaults`
- `service-intentions`
- `service-resolver`
- `service-router`
- `service-splitter`
- `tcp-route`
- `terminating-gateways`

### Envoy extension vocabulary

Consul's envoy extension vocabulary collects terms that describe

- `apigee`
- `ext`
- `lambda`
- `lua`
- `otel`
- `wasm`

### Enterprise vocabulary

Consul's enterprise vocabulary collects terms that describe

- `audit-log`
- `automated-backup`
- `automated-upgrade`
- `cts`
- `fips`
- `jwt-auth`
- `license`
- `lts`
- `network-area`
- `network-segment`
- `oidc-auth`
- `rate-limit`
- `read-replica`
- `redundancy-zone`

### Features vocabulary

Consul's features vocabulary collects terms that describe

- `acl`
- `certificate-authority`
- `cluster-peering`
- `consul-template`
- `discovery-chain`
- `distributed-tracing`
- `health-check`
- `intention`
- `kv`
- `load-balancing`
- `log`
- `mesh-gateway`
- `namespace`
- `prepared-query`
- `session`
- `snapshot`
- `static-lookup`
- `telemetry`
- `transparent-proxy`
- `wan-federation`
- `watch`

### Fundamentals vocabulary

Consul's fundamentals vocabulary collects terms that describe

- `install`
- `interface`
- `api`
- `cli`
- `env-var`
- `ui`
- `editions`
- `identity`

### Use case vocabulary

Consul's use case vocabulary collects terms that describe

- `service-discovery`
- `service-mesh`
- `hybrid`
- `multi-tenant`
- `automation`
- `configuration-as-code`
- `dns`

### Cross-product vocabulary

Cross-product vocabulary

### Content creation vocabulary

Content creation vocabulary

## Guide to Partials

Format -> Content block type -> Phase -> Feature -> Runtime
Examples -> Component -> Action -> Filetype

Partial structure describes what kind of content the partial is, then follows the existing filepath structures.

## How to update content

## How to build out content over time
