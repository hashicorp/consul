# Consul Documentation | Information Architecture and Content Strategy

The `website/content` directory in the `hashicorp/consul` repository contains [the Consul documentation on developer.hashicorp.com](https://developer.hashicorp.com/consul). This `README` describes the directory structure and design principles for this documentation set.

## Content directory overview

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
- Changes to `commands` appear at [https://developer.hashicorp.com/consul/commands](https://developer.hashicorp.com/consul/commands).
- Changes to `docs` appear at [https://developer.hashicorp.com/consul/docs](https://developer.hashicorp.com/consul/docs).

URLs follow the directory structure for each file and omit the the `.mdx` file extension. Pages named `index.mdx` adopt their directory's name. For example, the file `docs/reference/agent/index.mdx` appears at the URL [https://developer.hashicorp.com/consul/docs/reference/agent](https://developer.hashicorp.com/consul/docs/reference/agent).

The `partials` folder contains content that you can reuse across pages in any of the three folders. Refer to [Guide to Partials](#guide-to-partials) for more information.

Tutorials that appear at [https://developer.hashicorp.com/consul/tutorials](https://developer.hashicorp.com/consul/tutorials) are located in a different repository. This content exists in the [hashicorp/tutorials GitHub repo](https://github.com/hashicorp/tutorials), which is internal to the HashiCorp organization.

### Other directories of note

The `website/data` directory contains `.json` files that populate the navigation sidebar on [developer.hashicorp.com](https://developer.hashicorp.com).

The `website/public/img` directory contains the images used in the documentation.

Instructions on editing these files, including instructions on running local builds of the documentation, are in the `README` for the `website` directory, one level above this one.

## North Star principles

The design of the content in the `docs/` directory, including structure, file paths, and labels, is governed by the following _north star principles_.

1. **Users are humans**. Design for humans first. For example, file paths become URLs; create human-readable descriptions of the content and avoid unnecessary repetition.
1. **Less is always more**. Prefer single words for folder and file names; add a hyphen and a second word to disambiguate from existing content.
1. **Document what currently exists**. Do not create speculative folders and files to "reserve space" for future updates and releases. Do not describe Consul as it will exist in the future; describe it as it exists right now, in the latest release.
1. **Beauty works better**. When creating new files and directories, strive for consistency with the existing structure. For example, use parallel structures across directories and flatten directories that run too deep. Tip: If it doesn't look right, it's probably not right.
1. **Prefer partials over `ctrl+v`**. Spread content out, but document unique information in one place. When you need to repeat content across multiple pages, use partials to maintain content.

These principles exist to help you navigate ambiguity when making changes to the underlying content. If you add new content and you're not quite sure where to place it or how to name it, use these "north stars" to help you make an informed decision about what to do.

Over time, Consul may change in ways that require significant edits to this information architecture. The IA was designed with this possibility in mind. Use these north star principles to help you make informed (and preferably incremental) changes over time.

## Taxonomy

There are three main categories in the Consul docs information architecture. This division of categories is _not literal_ to the directory structure, even though the **Reference** category includes the repository's `reference` folder.

- Intro
- Usage
- Reference

These categories intentionally align with [Diataxis](https://diataxis.fr/).

### Intro

The **Intro** category includes the following folders in `website/content/docs/`:

- `architecture`
- `concept`
- `enterprise`
- `fundamentals`
- `use-case`

The following table lists each term and a definition to help you decide where to place new content.

| Term         | Directory      | What it includes                                                                                                                                                 |
| :----------- | :------------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Architecture | `architecture` | The product's components and their “maps” in cloud networking contexts.                                                                                          |
| Concepts     | `concept`      | Describes the complex behavior of technical systems in a non-literal manner. For example, computers do not literally “gossip” when they use the gossip protocol. |
| Enterprise   | `enterprise`   | Consul Enterprise license offerings and how to implement them.                                                                                                   |
| Fundamentals | `fundamentals` | The knowledge, connection and authorization methods, interactions, configurations, and operations most users require to use the product.                         |
| Use cases    | `use-case`     | The highest level goals practitioners have; a product function that “solves” enterprise concerns and usually competes with other products.                       |

### Usage

The **Usage** category includes the following folders in `website/content/docs/`:

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

These folders are organized into two groups that are _not literal_ to the directory structure, but are reflected in the navigation bar.

- **Operations**. User actions, workflows, and goals related to installing and operating Consul as a long-running daemon on multiple nodes in a network.
- **Service networking**. User actions, workflows, and goals related to networking solutions for application workloads.

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

Service Networking consists of the following phases, intentionally ordered to reflect a recommended “order of operations.” Although these phases do not need to be completed in order, their order reflects a general path for increasing complexity in Consul’s service networking capabilities as you develop your network.

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

The **Reference** category includes the following folders in `website/content/docs/`:

- `error-messages`
- `reference`
- `troubleshoot`

The following table lists each term and a definition to help you decide where to place new content.

| Term           | Directory         | What it includes                                                                                     |
| :------------- | :---------------- | :--------------------------------------------------------------------------------------------------- |
| Error Messages | `error-messsages` | Error messages and their causes, organized by runtime and Consul release binary.                     |
| Reference      | `reference`       | All reference information for configuring Consul, its components, and the infrastructure it runs on. |
| Troubleshoot   | `troubleshoot`    | Instructions and guidance about how to figure out what's wrong with a Consul deployment.             |

## Path syntax

A major advantage of this information architecture is the filepath structure. This structure "tags" documentation with keywords that describe the page's content to optimize the documentation for Google SEO while also helping human users build a "mental model" of Consul.

Our syntax creates human-readable names for file paths using a controlled vocabulary and intentional permutations. In general, the syntax follows a repeating pattern of `Verb / Noun / Adjective` to describe increasingly specific content and user goals.

For **Consul operations**, filepaths have the following order:

<CodeBlockConfig hideClipboard>

```plaintext
Phase -> Component -> Runtime -> Action -> Provider
```

</CodeBlockConfig>

Examples:

- `secure/encryption/tls/rotate/vm` contains instructions for rotating TLS certificates Consul agents use to secure their communications when running on virtual machines.
- `deploy/server/k8s/platform/openshift` contains instructions on deploying a Consul server agent when running OpenShift.
- `upgrade/k8s/compatibility` contains information about compatible software versions to help you upgrade the version of Consul running on Kubernetes.

For **service networking**, filepaths tend to have the following order:

<CodeBlockConfig hideClipboard>

```plaintext
Phase -> Feature -> Action -> Runtime -> Interface
```

</CodeBlockConfig>

Examples:

- `discover/load-balancer/nginx` contains instructions for using NGINX as a load balancer based on Consul service discovery results.
- `east-west/cluster-peering/establish/k8s` contains instructions for creating new connections between Consul clusters running on Kubernetes in different regions or cloud providers.
- `register/service/k8s/external` contains information about registering services running on external nodes to Consul on Kubernetes by configuring them to join the Consul datacenter.
- `register/external/k8s` contains information about registering services running on external nodes to Consul on Kubernetes with Consul External Services Manager (ESM).

## Controlled vocabularies

This section lists the standard names for files and directories, divded into sub-groups based on the descriptions in this `README`.

### Components vocabulary

Consul's _components_ vocabulary collects terms that describe Consul's built-in components, enterprise offerings, and other offerings that impact the operations of Consul agent clusters.

- `acl`
- `admin-partition`
- `audit-log`
- `automated-backup`
- `automated-upgrade`
- `auth-method`
- `cloud-auto-join`
- `cts`
- `dns`
- `fips`
- `jwt-auth`
- `license`
- `lts`
- `namespace`
- `network-area`
- `network-segment`
- `oidc-auth`
- `rate-limit`
- `read-replica`
- `redundancy-zone`
- `sentinel`
- `snapshot`
- `sso`

### Features vocabulary

Consul's _features_ vocabulary collects terms that describe Consul product offerings related to service networking for application workloads.

- `certificate`
- `cluster-peering`
- `consul-template`
- `dns`
- `discovery-chain`
- `distributed-tracing`
- `esm`
- `failover`
- `health-check`
- `intention`
- `ingress-gateway`
- `kv`
- `load-balancing`
- `log`
- `mesh-gateway`
- `prepared-query`
- `progressive-rollouts`
- `service`
- `session`
- `static-lookup`
- `telemetry`
- `transparent-proxy`
- `virtual-service`
- `wan-federation`
- `watch`

### Runtimes vocabulary

Consul's _runtimes_ vocabulary collects the underlying runtimes where Consul supports operations. This group includes provider-speicifc runtimes, such as EKS and AKS.

- `vm`
- `k8s`
- `nomad`
- `docker`
- `hcp`

#### Provider-specific runtimes

- `ecs`
- `eks`
- `lambda`
- `aks`
- `gks`
- `openshift`
- `argo`

### Actions vocabulary

Consul's _actions_ vocabulary collects the actions user take to operate Consul and enact service networking states.

- `backup-restore`
- `bootstrap`
- `configuration`
- `configure`
- `deploy`
- `enable`
- `encrypt`
- `forward`
- `initialize`
- `install`
- `listener`
- `manual`
- `migrate`
- `module`
- `monitor`
- `peer`
- `render`
- `requirements`
- `reroute`
- `rotate`
- `route`
- `run`
- `source`
- `store`
- `tech-specs`
- `troubleshoot`

### Providers vocabulary

Consul's _providers_ vocabulary collects the cloud providers and server locations that Consul runs on.

- `aws`
- `azure`
- `gcp`
- `external`
- `custom`

### Interfaces vocabulary

Consul's interfaces vocabulary includes the methods for interacting with Consul agents.

- `cli`
- `api`
- `ui`

### Architecture vocabulary

Consul's _architecture_ vocabulary is structured according to where components run:

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

Consul's _concepts_ vocabulary collects terms that describe how internal systems operate through human actions.

- `catalog`: Covers Consul's running record of the services it registered, including addresses and health check results.
- `consensus`: Covers the server agent elections governed by the Raft protocol.
- `consistency`: Covers Consul's anti-entropy features, consistency modes, and Jepsen testing.
- `gossip`: Covers Serf communication between Consul agents in a datacenter.
- `reliability`: Cover fault tolerance, quorum size, and server redundancy.

### Configuration entry vocabulary

Consul's _configuration entry_ vocabulary collects the names of the configuration entries and custom resource definitions (CRDs) that you must define to control service mesh state.

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

Consul's _Envoy extension_ vocabulary collects names of supported extensions that run on Envoy proxies.

- `apigee`
- `ext`
- `lambda`
- `lua`
- `otel`
- `wasm`

### Use case vocabulary

Consul's _use case_ vocabulary collects terms that describe

- `service-discovery`
- `service-mesh`
- `api-gateway`
- `config-management`
- `dns`

## Content strategy

This section describes the process for building out new content in the Consul documentation over time.

### Guide to Partials

Partials have file paths that begin by describing the type of content. Then, the filepath mirrors existing structures in the main docs folder. There are two syntaxes used for the partial filepaths:

<CodeBlockConfig hideClipboard>

```plaintext
Format -> Type -> Phase -> Feature -> Runtime
Examples -> Component -> Action -> Filetype
```

</CodeBlockConfig>

Reasons to use partials:

- You need to repeat the same information, such as steps or requirements, across runtimes or cloud providers
- You need to reference tables, especially ones that contain version numbers that are updated for each Consul release
- You need to include a configuration example that can be reused in both reference and usage contexts

### Document new Consul features

1. Create a file `name.mdx` that serves as an overview combining explanation, usage, and reference information.
2. When you need more pages, move the file to a folder with `name` and change the filename to `index.mdx`.
3. Create redirects as required.

For example, "DNS views" was introduced for Kubernetes in Consul v1.20. We created a file, `manage/dns/views.mdx`, then expanded it to `manage/dns/views/index.mdx` and `manage/dns/views/enable` when the content required separate pages. The first file is _always_ reachable at the URL `manage/dns/views`, despite the directory and filename change. The `k8s` label is not used because Kubernetes is the only runtime it supports. Hypothetically, if ECS support for DNS views became available, the directory structure for `content/docs/manage/dns` would become:

```
.
├── forwarding.mdx
└── views
    ├── enable
    |   ├── ecs.mdx
    |   └── k8s.mdx
    └── index.mdx
```

### Maintaining and deprecating content

Documentation is considered "maintained" when the usage instructions work on the oldest supported LTS release.

When components and features are no longer maintained, they may be "deprecated." To deprecate content:

1. Add a deprecation callout to the page. List the date or version when the deprecation occured.
1. On deprecation date, delete the content from the repository. Versioned docs preserves the information in older versions. If necessary, keep a single page in the documentation for announcement links and refirects.
1. Add redirects for deprecated content.
1. Move partials and images into a "legacy" folder if they are no longer used in the documentation.
