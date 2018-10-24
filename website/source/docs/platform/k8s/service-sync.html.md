---
layout: "docs"
page_title: "Service Sync - Kubernetes"
sidebar_current: "docs-platform-k8s-service-sync"
description: |-
  The services in Kubernetes and Consul can be automatically synced so that Kubernetes services are available to Consul agents and services in Consul can be available as first-class Kubernetes services.
---

# Syncing Kubernetes and Consul Services

The services in Kubernetes and Consul can be automatically synced so that Kubernetes
services are available to Consul agents and services in Consul can be available
as first-class Kubernetes services. This functionality is provided by the
[consul-k8s project](https://github.com/hashicorp/consul-k8s) and can be
automatically installed and configured using the
[Consul Helm chart](/docs/platform/k8s/helm.html).

**Why sync Kubernetes services to Consul?** Kubernetes services synced to the
Consul catalog enable Kubernetes services to be accessed by any node that
is part of the Consul cluster, including other distinct Kubernetes clusters.
For non-Kubernetes nodes, they can access services using the standard
[Consul DNS](/docs/agent/dns.html) or HTTP API.

**Why sync Consul services to Kubernetes?** Syncing Consul services to
Kubernetes services enables non-Kubernetes services (such as external to
the cluster) to be accessed in a native Kubernetes way: using kube-dns,
environment variables, etc. This makes it very easy to automate external
service discovery, including hosted services like databases.

## Installation and Configuration

The service sync is done using an external long-running process in the
[consul-k8s project](https://github.com/hashicorp/consul-k8s). This process
can run either in or out of a Kubernetes cluster. However, running this within
the Kubernetes cluster is generally easier since it is automated using the
[Helm chart](/docs/platform/k8s/helm.html).

The Consul server cluster can run either in or out of a Kubernetes cluster.
The Consul server cluster does not need to be running on the same machine
or same platform as the sync process. The sync process needs to be configured
with the address to the Consul cluster as well as any additional access
information such as ACL tokens.

To install the sync, enable the catalog sync feature using
[Helm values](/docs/platform/k8s/helm.html#configuration-values-) and
upgrade the installation using `helm upgrade` for existing installs or
`helm install` for a fresh install.

```yaml
syncCatalog:
  enabled: true
```

This will enable services to sync _in both directions_. You can also choose
to only sync Kubernetes services to Consul or vice versa by disabling a direction.
See the [Helm configuration](/docs/platform/k8s/helm.html#configuration-values-)
for more information.

-> **Before installing,** please read the introduction paragraphs for the
reference documentation below for both
[Kubernetes to Consul](/docs/platform/k8s/service-sync.html#kubernetes-to-consul) and
[Consul to Kubernetes](/docs/platform/k8s/service-sync.html#consul-to-kubernetes)
sync to understand how the syncing works.

### Authentication

The sync process must authenticate to both Kubernetes and Consul to read
and write services.

For Consul, the process accepts both the standard CLI flag `-token` and
the environment variable `CONSUL_HTTP_TOKEN`. This should be set to an
Consul [ACL token](/docs/guides/acl.html) if ACLs are enabled. This
can also be configured using the Helm chart to read from a Kubernetes
secret.

For Kubernetes, a valid kubeconfig file must be provided with cluster
and auth information. The sync process will look into the default locations
for both in-cluster and out-of-cluster authentication. If `kubectl` works,
then the sync program should work.

## Kubernetes to Consul

This sync registers Kubernetes services to the Consul catalog automatically.

This enables discovery and connection to Kubernetes services using native
Consul service discovery such as DNS or HTTP. This is particularly useful for
non-Kubernetes nodes. This also causes all discoverable services to be part of
a central service catalog in Consul for further syncing into alternate
Kubernetes clusters or other platforms.

### Kubernetes Service Types

Not all Kubernetes services are externally accessible. The sync program by
default will only sync services of the following types or configurations.
If a service type is not listed below, then the sync program will ignore that
service type.

#### NodePort

[NodePort services](https://kubernetes.io/docs/concepts/services-networking/service/#nodeport)
register a static port that every node in the K8S cluster listens on.

For NodePort services, a Consul service instance will be created for each
node that has the representative pod running. While Kubernetes configures
a static port on all nodes in the cluster, this limits the number of service
instances to be equal to the nodes running the target pods.

The service instances will be registered to the Kubernetes node name
that each instance lives on. This is guaranteed unique by Kubernetes. An
existing node entry will be used if it is already part of the Consul
cluster (for example if you're running a client agent on all Kubernetes
nodes). This allows the normal agent health checks for that node to continue
working.

#### LoadBalancer

For LoadBalancer services, a single service instance will be registered with
the external IP of the created load balancer. Because this is already a load
balancer, only one service instance will be registered with Consul rather
than registering each individual pod endpoint.

#### External IPs

Any service type may specify an
"[external IP](https://kubernetes.io/docs/concepts/services-networking/service/#external-ips)"
configuration. The external IP must be configured by some other system, but
any service discovery will resolve to this set of IP addresses rather than a
virtual IP.

If an external IP list is present, a service instance in Consul will be created
for each external IP. It is assumed that if an external IP is present that it
is routable and configured by some other system.

### Sync Enable/Disable

By default, all valid services (as explained above) are synced. This default
can be changed as configuration to the sync process. Syncing can also be
explicitly enabled or disabled using an annotation:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: my-service
  annotations:
    "consul.hashicorp.com/service-sync": false
```

### Service Name

When a Kubernetes service is synced to Consul, the name of the service in Consul
by default will be the value of the "name" metadata on that Kubernetes service.
This makes it so that service sync works with zero configuration changes.
This can be overridden using an annotation to specify the Consul service name:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: my-service
  annotations:
    "consul.hashicorp.com/service-name": my-consul-service
```

**If a conflicting service name exists in Consul,** the sync program will
register additional instances to that same service. Therefore, services inside
and outside of Kubernetes should have different names unless you want either
side to potentially connect. This default behavior also enables gracefully
transitioning a service from outside of K8S to inside, and vice versa.

### Service Ports

When syncing the Kubernetes service to Consul, the Consul service port will be
the first defined port in the service. Additionally, all ports will be
registered in the service instance metadata with the key "port-X" where X is
the name of the port and the value is the externally accessible port.

The default service port can be overridden using an annotation:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: my-service
  annotations:
    "consul.hashicorp.com/service-port": "http"
```

The annotation value may a name of a port (recommended) or an exact port value.

### Service Tags

A service registered in Consul from Kubernetes will always have the tag "k8s" added
to it. Additional tags can be specified with a comma-separated annotation value
as shown below. This will also automatically include the "k8s" tag which can't
be disabled. The values should be specified comma-separated without any
additional whitespace.

```yaml
kind: Service
apiVersion: v1
metadata:
  name: my-service
  annotations:
    "consul.hashicorp.com/service-tags": "primary,foo"
```

### Service Meta

A service registered in Consul from Kubernetes will set the `external-source` key to
"kubernetes". This can be used by API consumers, the UI, CLI, etc. to filter
service instances that are set in k8s. The Consul UI (in Consul 1.2.3 and later)
will read this value to show a Kubernetes icon next to all externally
registered services from Kubernetes.

Additional metadata can be specified using annotations. The "KEY" below can be
set to any key. This allows setting multiple meta values:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: my-service
  annotations:
    "consul.hashicorp.com/service-meta-KEY": "value"
```

## Consul to Kubernetes

This syncs Consul services into first-class Kubernetes services.
Each Consul service is synced to an
[ExternalName](https://kubernetes.io/docs/concepts/services-networking/service/#externalname)
service in Kubernetes. The external name is configured to be the Consul
DNS entry.

This enables external services to be discovered using native Kubernetes
tooling. This can be used to ease software migration into or out of Kubernetes,
across platforms, to and from hosted services, and more.

-> **Requires Consul DNS via CoreDNS in Kubernetes:** This feature requires that
[Consul DNS](/docs/platform/k8s/dns.html) is configured within Kubernetes.
Additionally, **[CoreDNS](https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/#config-coredns)
is required (instead of kube-dns)** to resolve an
issue with resolving `externalName` services pointing to custom domains.
In the future we hope to remove this requirement by syncing the instance
addresses directly into service endpoints.

### Sync Enable/Disable

All Consul services visible to the sync process based on its given ACL token
will be synced to Kubernetes.

There is no way to change this behavior per service. For the opposite sync
direction (Kubernetes to Consul), you can use Kubernetes annotations to disable
a sync per service. This is not currently possible for Consul to Kubernetes
sync and the ACL token must be used to limit what services are synced.

In the future, we hope to support per-service configuration.

### Service Name

When a Consul service is synced to Kubernetes, the name of the Kubernetes
service will exactly match the name of the Consul service.

To change this default exact match behavior, it is possible to specify a
prefix to be added to service names within Kubernetes by using the
`-k8s-service-prefix` flag. This can also be specified in the Helm
configuration.

**If a conflicting service is found,** the service will not be synced. This
does not match the Kubernetes to Consul behavior, but given the current
implementation we must do this because Kubernetes can't mix both CNAME and
Endpoint-based services.

### Kubernetes Service Labels and Annotations

Any Consul services synced to Kubernetes will be labeled and annotated.
An annotation `consul.hashicorp.com/synced` will be set to "true" to note
that this is a synced service from Consul.

Additionally, a label `consul=true` will be specified so that label selectors
can be used with `kubectl` and other tooling to easily filter all Consul-synced
services.

