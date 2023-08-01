# Consul Developer Documentation

See [our contributing guide](../.github/CONTRIBUTING.md) to get started.

This directory contains documentation intended for anyone interested in
understanding, and contributing changes to, the Consul codebase.

## Overview

This documentation is organized into the following categories. Each category is 
either a significant architectural layer, or major functional area of Consul. 
These documents assume a basic understanding of Consul's feature set, which can
be found in the public [user documentation].

[user documentation]: https://developer.hashicorp.com/consul/docs

![Overview](./overview.svg)

<sup>[source](./overview.mmd)</sup>

## Contents 

1. [Command-Line Interface (CLI)](./cli)
1. [HTTP API](./http-api)
1. [Agent Configuration](./config)
1. [RPC](./rpc)
1. [Cluster Persistence](./persistence)
1. [Resources and Controllers](./resources)
1. [Client Agent](./client-agent)
1. [Service Discovery](./service-discovery)
1. [Service Mesh (Connect)](./service-mesh)
1. [Cluster Membership](./cluster-membership)
1. [Key/Value Store](./kv)
1. [ACL](./acl)
1. [Multi-Cluster Federation](./cluster-federation)

Also see the [FAQ](./faq.md).

## Other Docs

1. [Integration Tests](../test/integration/connect/envoy/README.md)
1. [Upgrade Tests](../test/integration/consul-container/test/upgrade/README.md)
1. [Remote Debugging Integration Tests](../test/integration/consul-container/test/debugging.md)
1. [Peering Common Topology Tests](../test-integ/peering_commontopo/README.md)

## Important Directories

Most top level directories contain Go source code. The directories listed below
contain other important source related to Consul.

* [ui] contains the source code for the Consul UI.
* [website] contains the source for [consul.io](https://www.consul.io/). A pull requests
  can update the source code and Consul's documentation at the same time.
* [.github] contains the source for our CI and GitHub repository
  automation.
* [.changelog] contains markdown files that are used by [hashicorp/go-changelog] to produce the
  [CHANGELOG.md].
* [build-support] contains bash functions and scripts used to automate.
  development tasks. Generally these scripts are called from the [Makefile].
* [grafana] contains the source for a [Grafana dashboard] that can be used to
  monitor Consul.

[ui]: https://github.com/hashicorp/consul/tree/main/ui
[website]: https://github.com/hashicorp/consul/tree/main/website
[.github]: https://github.com/hashicorp/consul/tree/main/.github
[.changelog]: https://github.com/hashicorp/consul/tree/main/.changelog
[hashicorp/go-changelog]: https://github.com/hashicorp/go-changelog
[CHANGELOG.md]: https://github.com/hashicorp/consul/blob/main/CHANGELOG.md
[build-support]: https://github.com/hashicorp/consul/tree/main/build-support
[Makefile]: https://github.com/hashicorp/consul/tree/main/Makefile
[Grafana dashboard]: https://grafana.com/grafana/dashboards
[grafana]: https://github.com/hashicorp/consul/tree/main/grafana


## Contributing to these docs

This section is meta documentation about contributing to these docs.

### Diagrams

The diagrams in these documents are created using the [mermaid-js live editor]. 
The [mermaid-js docs] provide a complete reference for how to create and edit 
the diagrams. Use the [consul-mermaid-theme.json] (paste it into the Config tab 
in the editor) to maintain a consistent Consul style for the diagrams.

[mermaid-js live editor]: https://mermaid-js.github.io/mermaid-live-editor/edit/
[mermaid-js docs]: https://mermaid-js.github.io/mermaid/
[consul-mermaid-theme.json]: ./consul-mermaid-theme.json
