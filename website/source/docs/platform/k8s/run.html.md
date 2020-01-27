---
layout: "docs"
page_title: "Installing Consul on Kubernetes - Kubernetes"
sidebar_current: "docs-platform-k8s-run"
description: |-
  Consul can run directly on Kubernetes, both in server or client mode. For pure-Kubernetes workloads, this enables Consul to also exist purely within Kubernetes. For heterogeneous workloads, Consul agents can join a server running inside or outside of Kubernetes.
---

# Installing Consul on Kubernetes

Consul can run directly on Kubernetes, both in server or client mode.
For pure-Kubernetes workloads, this enables Consul to also exist purely
within Kubernetes. For heterogeneous workloads, Consul agents can join
a server running inside or outside of Kubernetes.

This page starts with a large how-to section for various specific tasks.
To learn more about the general architecture of Consul on Kubernetes, scroll
down to the [architecture](/docs/platform/k8s/run.html#architecture) section.
If you would like to get hands-on experience testing Consul on Kubernetes,
try the step-by-step beginner tutorial with an accompanying video in the
[Minikube with Consul guide](https://learn.hashicorp.com/consul/getting-started-k8s/minikube?utm_source=consul.io&utm_medium=docs)

## Helm Chart Installation

The recommended way to run Consul on Kubernetes is via the
[Helm chart](/docs/platform/k8s/helm.html). This will install and configure
all the necessary components to run Consul. The configuration enables you
to run just a server cluster, just a client cluster, or both. Using the Helm
chart, you can have a full Consul deployment up and running in minutes.

A step-by-step beginner tutorial and accompanying video can be found at the
[Minikube with Consul guide](https://learn.hashicorp.com/consul/getting-started-k8s/minikube?utm_source=consul.io&utm_medium=docs).

While the Helm chart exposes dozens of useful configurations and automatically
sets up complex resources, it **does not automatically operate Consul.**
You are still responsible for learning how to monitor, backup,
upgrade, etc. the Consul cluster.

The Helm chart has no required configuration and will install a Consul
cluster with sane defaults out of the box. Prior to going to production,
it is highly recommended that you
[learn about the configuration options](/docs/platform/k8s/helm.html#configuration-values-).

~> **Security Warning:** By default, the chart will install an insecure configuration
of Consul. This provides a less complicated out-of-box experience for new users,
but is not appropriate for a production setup. It is highly recommended to use
a properly secured Kubernetes cluster or make sure that you understand and enable
the [recommended security features](/docs/internals/security.html). Currently,
some of these features are not supported in the Helm chart and require additional
manual configuration.

### Prerequisites

The Consul Helm chart works with Helm 2 and Helm 3. If using Helm 2, you will
need to install Tiller by following the
[Helm 2 Installation Guide](https://v2.helm.sh/docs/using_helm/#quickstart-guide).

### Installing Consul

Determine the latest version of the Consul Helm chart
by visiting [https://github.com/hashicorp/consul-helm/releases](https://github.com/hashicorp/consul-helm/releases).

Clone the chart at that version. For example, if the latest version is
`v0.8.1`, you would run:

```bash
$ git clone --single-branch --branch v0.8.1 https://github.com/hashicorp/consul-helm.git
Cloning into 'consul-helm'...
...
You are in 'detached HEAD' state...
```

Ensure you've checked out the correct version with `helm inspect chart`:

```bash
$ helm inspect chart ./consul-helm
apiVersion: v1
description: Install and configure Consul on Kubernetes.
home: https://www.consul.io
name: consul
sources:
- https://github.com/hashicorp/consul
- https://github.com/hashicorp/consul-helm
- https://github.com/hashicorp/consul-k8s
version: 0.8.1
```

Now you're ready to install Consul! To install Consul with the default
configuration using Helm 3 run:

```sh
$ helm install hashicorp ./consul-helm
NAME: hashicorp
...
```

-> If using Helm 2, run: `helm install --name hashicorp ./consul-helm`

_That's it._ The Helm chart does everything to set up a recommended
Consul-on-Kubernetes deployment.
In a couple minutes, a Consul cluster will be formed and a leader
elected and every node will have a running Consul agent.

### Customizing Your Installation
If you want to customize your installation,
create a `config.yaml` file to override the default settings.
You can learn what settings are available by running `helm inspect values ./consul-helm`
or by reading the [Helm Chart Reference](/docs/platform/k8s/helm.html).

Once you've created your `config.yaml` file, run `helm install` with the `-f` flag:

```bash
$ helm install hashicorp ./consul-helm -f config.yaml
```

If you've already installed Consul and want to make changes, you'll need to run
`helm upgrade`. See the [Upgrading Consul on Kubernetes](/docs/platform/k8s/run.html#upgrading-consul-on-kubernetes)
section for more details.

## Viewing the Consul UI

The Consul UI is enabled by default when using the Helm chart.
For security reasons, it isn't exposed via a `LoadBalancer` Service by default so you must
use `kubectl port-forward` to visit the UI: 

```
$ kubectl port-forward service/hashicorp-consul-server 8500:8500
...
```

Once the port is forwarded navigate to [http://localhost:8500](http://localhost:8500).

If you want to expose the UI via a Kubernetes Service, configure
the [`ui.service` chart values](/docs/platform/k8s/helm.html#v-ui-service).
This service will allow requests to the Consul servers so it should
not be open to the world.

## Accessing the Consul HTTP API

The Consul HTTP API should be accessed by communicating to the local agent
running on the same node. While technically any listening agent (client or
server) can respond to the HTTP API, communicating with the local agent
has important caching behavior, and allows you to use the simpler
[`/agent` endpoints for services and checks](/api/agent.html).

For Consul installed via the Helm chart, a client agent is installed on
each Kubernetes node. This is explained in the [architecture](/docs/platform/k8s/run.html#client-agents)
section. To access the agent, you may use the
[downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/).

An example pod specification is shown below. In addition to pods, anything
with a pod template can also access the downward API and can therefore also
access Consul: StatefulSets, Deployments, Jobs, etc.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: consul-example
spec:
  containers:
    - name: example
      image: "consul:latest"
      env:
        - name: HOST_IP
          valueFrom:
            fieldRef:
              fieldPath: status.hostIP
      command:
        - "/bin/sh"
        - "-ec"
        - |
            export CONSUL_HTTP_ADDR="${HOST_IP}:8500"
            consul kv put hello world
  restartPolicy: Never
```

An example `Deployment` is also shown below to show how the host IP can
be accessed from nested pod specifications:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: consul-example-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: consul-example
  template:
    metadata:
      labels:
        app: consul-example
    spec:
      containers:
        - name: example
          image: "consul:latest"
          env:
            - name: HOST_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.hostIP
          command:
            - "/bin/sh"
            - "-ec"
            - |
                export CONSUL_HTTP_ADDR="${HOST_IP}:8500"
                consul kv put hello world
```


## Architecture

We recommend running Consul on Kubernetes with the same
[general architecture](/docs/internals/architecture.html)
as running it anywhere else. There are some benefits Kubernetes can provide
that eases operating a Consul cluster and we document those below. The standard
[production deployment guide](https://learn.hashicorp.com/consul/datacenter-deploy/deployment-guide) is still an
important read even if running Consul within Kubernetes.

Each section below will outline the different components of running Consul
on Kubernetes and an overview of the resources that are used within the
Kubernetes cluster.

### Server Agents

The server agents are run as a **StatefulSet**, using persistent volume
claims to store the server state. This also ensures that the
[node ID](/docs/agent/options.html#_node_id) is persisted so that servers
can be rescheduled onto new IP addresses without causing issues. The server agents
are configured with
[anti-affinity](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity)
rules so that they are placed on different nodes. A readiness probe is
configured that marks the pod as ready only when it has established a leader.

A **Service** is registered to represent the servers and expose the various
ports. The DNS address of this service is used to join the servers to each
other without requiring any other access to the Kubernetes cluster. The
service is configured to publish non-ready endpoints so that it can be used
for joining during bootstrap and upgrades.

Additionally, a **PodDisruptionBudget** is configured so the Consul server
cluster maintains quorum during voluntary operational events. The maximum
unavailable is `(n/2)-1` where `n` is the number of server agents.

-> **Note:** Kubernetes and Helm do not delete Persistent Volumes or Persistent
Volume Claims when a
[StatefulSet is deleted](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#stable-storage),
so this must done manually when removing servers.

### Client Agents

The client agents are run as a **DaemonSet**. This places one agent
(within its own pod) on each Kubernetes node.
The clients expose the Consul HTTP API via a static port (default 8500)
bound to the host port. This enables all other pods on the node to connect
to the node-local agent using the host IP that can be retrieved via the
Kubernetes downward API. See
[accessing the Consul HTTP API](/docs/platform/k8s/run.html#accessing-the-consul-http-api)
for an example.

There is a major limitation to this: there is no way to bind to a local-only
host port. Therefore, any other node can connect to the agent. This should be
considered for security. For a properly production-secured agent with TLS
and ACLs, this is safe.

Some people prefer to run **Consul agent per pod** architectures, since this
makes it easy to register the pod as a service. However, this turns
a pod into a "node" in Consul and also causes an explosion of resource usage
since every pod needs a Consul agent. We recommend instead running an
agent (in a dedicated pod) per node, via the DaemonSet. This maintains the
node equivalence in Consul. Service registration should be handled via the
catalog syncing feature with Services rather than pods.

-> **Note:** Due to a limitation of anti-affinity rules with DaemonSets,
a client-mode agent runs alongside server-mode agents in Kubernetes. This
duplication wastes some resources, but otherwise functions perfectly fine.
