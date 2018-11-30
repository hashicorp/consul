---
layout: "docs"
page_title: "Consul Template"
sidebar_current: "docs-guides-consul-template"
description: |-
  Consul template provides a programmatic method for rendering configuration files from Consul data.
---

# Consul Template

The Consul template tool provides a programmatic method
for rendering configuration files from a variety of locations,
including Consul KV. It is an ideal option for replacing complicated API
queries that often require custom formatting.
The template tool is based on Go templates and shares many
of the same attributes.

Consul template is a useful tool with several uses, we will focus on two
of it's use cases.

1. *Update configuration files*. The Consul template tool can be used
to update service configuration files. A common use case is managing load
balancer configuration files that need to be updated regularly in a dynamic
infrastructure on machines many not be able to directly connect to the Consul cluster.

1. *Discover data about the Consul cluster and service*. It is possible to collect
information about the services in your Consul cluster. For example, you could
collect a list of all services running on the cluster or you could discover all
service addresses for the Redis service. Note, this use case has limited
scope for production.

In this guide we will briefly discuss how `consul-template` works,
how to install it, and two use cases.

Before completing this guide, we assume some familiarity with
[Consul KV](https://learn.hashicorp.com/consul/getting-started/kv)
 and [Go templates](https://golang.org/pkg/text/template/).

## Introduction to Consul Template

Consul template is a simple, yet powerful tool. When initiated, it
reads one or more template files and queries Consul for all
data needed to render them. Typically, you run `consul-template` as a
daemon which will fetch the initial values and then continue to watch
for updates, re-rendering the template whenever there are relevant changes in
the cluster. You can alternatively use the `-once` flag to fetch and render
the template once which is useful for testing and
setup scripts that are triggered by some other automation for example a
provisioning tool. Finally, the template can also run arbitrary commands after the update
process completes. For example, it can send the HUP signal to the
load balancer service after a configuration change has been made.

The Consul template tool is flexible, it can fit into many
different environments and workflows. Depending on the use-case, you
may have a single `consul-template` instance on a handful of hosts
or may need to run several instances on every host. Each `consul-template`
process can manage multiple unrelated files though and will de-duplicate
 the fetches as needed if those files share data dependencies so it can
reduce the load on Consul servers to share where possible.

## Install Consul Template

For this guide, we are using a local Consul agent in development
mode which can be started with `consul agent -dev`. To quickly set
up a local Consul agent, refer to the getting started [guide](https://learn.hashicorp.com/consul/getting-started/install). The
Consul agent must be running to complete all of the following
steps.

The Consul template tool is not included with the Consul binary and will
need to be installed separately. It can be installed from a precompiled
binary or compiled from source. We will be installing the precompiled binary.

First, download the binary from the [Consul Template releases page](https://releases.hashicorp.com/consul-template/).

```sh
curl -O https://releases.hashicorp.com/consul-template/0.19.5/consul-template<_version_OS>.tgz
```

Next, extract the binary and move it into your `$PATH`.

```sh
tar -zxf consul-template<_version_OS>.tgz
```

To compile from source, please see the instructions in the
[contributing section in GitHub](https://github.com/hashicorp/consul-template#contributing).

## Use Case: Consul KV

In this first use case example, we will render a template that pulls the HashiCorp address
from Consul KV. To do this we will create a simple template that contains the HashiCorp
address, run `consul-template`, add a value to Consul KV for HashiCorp's address, and
finally view the rendered file.

First, we will need to create a template file `find_address.tpl` to query
Consul's KV store:

```liquid
{{ key "/hashicorp/street_address" }}
```

Next, we will run `consul-template` specifying both
the template to use and the file to update.

```shell
$ consul-template -template "find_address.tpl:hashicorp_address.txt"
```

The `consul-template` process will continue to run until you kill it with `CRTL+c`.
For now, we will leave it running.

Finally, open a new terminal so we can write data to the key in Consul using the command
line interface.

```shell
$ consul kv put hashicorp/street_address "101 2nd St"

Success! Data written to: hashicorp/street_address
```

We can ensure the data was written by viewing the `hashicorp_address.txt`
file which will be located in the same directory where `consul-template`
was run.

```shell
$ cat hashicorp_address.txt

101 2nd St
```

If you update the key `hashicorp/street_address`, you can see the changes to the file
immediately. Go ahead and try `consul kv put hashicorp/street_address "22b Baker ST"`.

You can see that this simple process can have powerful implications. For example, it is
possible to use this same process for updating your [HAProxy load balancer
configuration](https://github.com/hashicorp/consul-template/blob/master/examples/haproxy.md).

You can now kill the `consul-template` process with `CTRL+c`.

## Use Case: Discover All Services

In this use case example, we will discover all the services running in the Consul cluster.
To follow along, you use the local development agent from the previous example.

First, we will need to create a new template `all-services.tpl` to query all services.

```liquid
{{range services}}# {{.Name}}{{range service .Name}}
{{.Address}}{{end}}

{{end}}
```

Next, run Consul template specifying the template we just created and the `-once` flag.
The `-once` flag will tell the process to run once and then quit.

```shell
$ consul-template -template="all-services.tpl:all-services.txt" -once
```

If you complete this on your local development agent, you should
still see the `consul` service when viewing `all-services.txt`.

```text
# consul
127.0.0.7
```
On a development or production cluster, you would see a list of all the services.
For example:

```text
# consul
104.131.121.232

# redis
104.131.86.92
104.131.109.224
104.131.59.59

# web
104.131.86.92
104.131.109.224
104.131.59.59
```

## Conclusion

In this guide we learned how to set up and use the Consul template tool.
To see additional examples, refer to the examples folder
in [GitHub](https://github.com/hashicorp/consul-template/tree/master/examples).
