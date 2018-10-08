---
layout: "docs"
page_title: "Connect Sidecar - Kubernetes"
sidebar_current: "docs-platform-k8s-connect"
description: |-
  Connect is a feature built-in to Consul that enables automatic service-to-service authorization and connection encryption across your Consul services. Connect can be used with Kubernetes to secure pod communication with other services.
---

# Connect Sidecar on Kubernetes

[Connect](/docs/connect/index.html) is a feature built-in to Consul that enables
automatic service-to-service authorization and connection encryption across
your Consul services. Connect can be used with Kubernetes to secure pod
communication with other services.

Consul can automatically inject [Envoy as a sidecar](#)
into pods in your cluster. This makes Connect configuration for Kubernetes automatic.
This functionality is provided by the
[consul-k8s project](https://github.com/hashicorp/consul-k8s) and can be
automatically installed and configured using the
[Consul Helm chart](/docs/platform/k8s/helm.html).

## Usage

When the
[Connect injector is installed](/docs/platform/k8s/connect.html#installation-and-configuration),
the Connect sidecar is automatically added to pods. This sidecar can both
accept and establish connections using Connect, enabling the pod to communicate
to clients and dependencies exclusively over authorized and encrypted
connections.

-> **Note:** The pod specifications in this section are valid and use
publicly available images. If you've installed the Connect injector, feel free
to run the pod specifications in this section to try Connect with Kubernetes.

### Accepting Inbound Connections

An example pod is shown below with Connect enabled to accept inbound
connections. Notice that the pod would still be fully functional without
Connect. Minimal to zero modifications are required to pod specifications to
enable Connect in Kubernetes.

This pod specification starts an "echo" server that responds to any
HTTP request with the static text "hello world".

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: echo-server
  annotations:
    "consul.hashicorp.com/connect-inject": "true"
spec:
  containers:
    - name: echo-server
      image: hashicorp/http-echo:latest
      args:
        - -text="hello world"
        - -listen=:8080
      ports:
        - containerPort: 8080
          name: http
```

The only change for Connect is the addition of the
`consul.hashicorp.com/connect-inject` annotation. This enables injection
for this pod. The injector can also be
[configured](/docs/platform/k8s/connect.html#installation-and-configuration)
to automatically inject unless explicitly disabled, but the default
installation requires opt-in using the annotation shown above.

This will start a Connect sidecar that listens on a random port registered
with Consul and proxies valid inbound connections to port 8080 in the pod.
To establish a connection to the pod, a client must use another Connect
proxy. The client Connect proxy will use Consul service discovery to find
all available upstream proxies and their public ports.

In the example above, the server is listening on `:8080`. This means
the server will still bind to the pod IP and allow external connections.
This is useful to transition to Connect by allowing both Connect and
non-Connect connections. To restrict only Connect connections, any listeners
should bind to localhost only (such as `127.0.0.1`).

### Connecting to Connect-Enabled Services

The example pod specification below configures a pod that is capable
of establishing connections to our previous example "echo" service. The
connection to this echo service happens over an authorized and encrypted
connection via Connect.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: echo-client
  annotations:
    "consul.hashicorp.com/connect-inject": "true"
    "consul.hashicorp.com/connect-service-upstreams": "echo-server:1234"
spec:
  containers:
    - name: echo-client
      image: tutum/curl:latest
      # Just spin & wait forever, we'll use `kubectl exec` to demo
      command: [ "/bin/sh", "-c", "--" ]
      args: [ "while true; do sleep 30; done;" ]
```

Pods must specify upstream dependencies with the
[`consul.hashicorp.com/connect-service-upstreams` annotation](/docs/platform/k8s/connect.html#consul-hashicorp-com-connect-service-upstreams).
This annotation declares the names of any upstream dependencies and a
local port to listen on. When a connection is established to that local
port, the proxy establishes a connection to the target service
("echo-server" in this example) using
mutual TLS and identifying as the source service ("echo-client" in this
example).
Any containers running in the pod that need to establish connections
to dependencies must be reconfigured to use the local upstream address.
This means pods should not use Kubernetes service DNS or environment
variables for these connections.

We can verify access to the echo server using `kubectl exec`. Notice
that we `curl` the local address and local port 1234 specified with our
upstreams.

```sh
$ kubectl exec echo-client -- curl -s http://127.0.0.1:1234/
"hello world"
```

If you use the Consul UI or [CLI](/docs/commands/intention/create.html) to
create a deny [intention](/docs/connect/intentions.html) between
"echo-client" and "echo-server", connections are immediately rejected
without updating either of the running pods. You can then remove this
intention to allow connections again.

```sh
$ kubectl exec echo-client -- curl -s http://127.0.0.1:1234/
command terminated with exit code 52
```

### Available Annotations

Annotations can be used to configure the injection behavior.

* `consul.hashicorp.com/connect-inject` - If this is "true" then injection
  is enabled. If this is "false" then injection is explicitly disabled.
  The default is configurable on the injector itself. When installing
  the injector, the default behavior requires pod specify this to opt-in to
  injection.

* `consul.hashicorp.com/connect-service` - For pods that accept inbound
  connections, this specifies the name of the service that is being
  served. This defaults to the name of the first container in the pod.

* `consul.hashicorp.com/connect-service-port` - For pods that accept inbound
  connections, this specifies the port to route inbound connections to. This
  is the port that the service is listening on. The proxy will listen on
  a dynamic port. This defaults to the first exposed port on any container
  in the pod. If specified, the value can be the _name_ of a configured port,
  such as "http" or it can be a direct port value such as "8080".

* `consul.hashicorp.com/connect-service-upstreams` - The list of upstream
  services that this pod needs to connect to via Connect along with a static
  local port to listen for those connections. Example: `db:1234,auth:6789`
  will start two local listeners for `db` on port 1234 and `auth` on port
  6789, respectively. The name of the service is the name of the service
  registered with Consul. This value defaults to no upstreams.

### Deployments, StatefulSets, etc.

The annotations for configuring Connect must be on the pod specification.
Since higher level resources such as Deployments wrap pod specification
templates, Connect can be used with all of these higher level constructs, too.

An example `Deployment` below shows how to enable Connect injection:

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
      annotations:
        "consul.hashicorp.com/connect-inject": "true"
    spec:
      containers:
        - name: example
          image: "nginx"
```

~> **A common mistake** is to set the annotation on the Deployment or
other resource. Ensure that the injector annotations are specified on
the _pod specification template_ as shown above.

## Installation and Configuration

The Connect sidecar proxy is injected via a
[mutating admission webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#admission-webhooks)
provided by the
[consul-k8s project](https://github.com/hashicorp/consul-k8s).
This enables the automatic pod mutation shown in the usage section above.
Installation of the mutating admission webhook is automated using the
[Helm chart](/docs/platform/k8s/helm.html).

To install the Connect injector, enable the Connect injection feature using
[Helm values](/docs/platform/k8s/helm.html#configuration-values-) and
upgrade the installation using `helm upgrade` for existing installs or
`helm install` for a fresh install. The Connect injector **also requires**
[client agents](/docs/platform/k8s/helm.html#v-client) are enabled on
the node with pods that are using Connect and that
[gRPC is enabled](/docs/platform/k8s/helm.html#v-client-grpc).

```yaml
connectInject:
  enabled: true

client:
  enabled: true
  grpc: true
```

This will configure the injector to inject when the
[injection annotation](#)
is present. Other values in the Helm chart can be used to limit the namespaces
the injector runs in, enable injection by default, and more.

As noted above, the Connect auto-injection requires that local client agents
are configured. These client agents must be successfully joined to a Consul
cluster.
The Consul server cluster can run either in or out of a Kubernetes cluster.

### Verifying the Installation

To verify the installation, run the
["Accepting Inbound Connections"](/docs/platform/k8s/connect.html#accepting-inbound-connections)
example from the "Usage" section above. After running this example, run
`kubectl get pod echo-server -o yaml`. In the raw YAML output, you should
see injected Connect containers and an annotation
`consul.hashicorp.com/connect-inject-status` set to `injected`. This
confirms that injection is working properly.

If you do not see this, then use `kubectl logs` against the injector pod
and note any errors.
