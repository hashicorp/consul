---
layout: "docs"
page_title: "Mesh Gateways - Kubernetes (beta)"
sidebar_current: "docs-platform-k8s-mesh-gateways"
description: |-
    This documentation describes how to configure Connect Mesh Gateways in Kubernetes.
---

# Mesh Gateways on Kubernetes <sup>(beta)</sup>

Mesh gateways enable routing of Connect traffic between different Consul datacenters.
See [Mesh Gateways](/docs/connect/mesh_gateway.html) for more details.

To deploy Gateways in Kubernetes, use our [consul-helm](/docs/platform/k8s/helm.html) chart.

## Usage
There are many configuration options for gateways nested under the [`meshGateway`](/docs/platform/k8s/helm.html#v-meshgateway) key
of the consul-helm chart:

```yaml
meshGateway:
  enabled: true
```

See the [chart reference](/docs/platform/k8s/helm.html#v-meshgateway) for full documentation
on all the options.

## Architecture
Gateways run as `Pod`s in a `Deployment`. They forward requests from other datacenters
through to the correct services running in Kubernetes.

Since gateways need to be accessed from other datacenters they will need to be exposed. The
recommended way to do this is via a load balancer.

For information on the general architecture of mesh gateways see [Mesh Gateways](/docs/connect/mesh_gateway.html).

## Guide to Using Mesh Gateways To Connect Two Kubernetes Clusters
This guide shows how to configure the consul-helm chart to set up gateways
that route Connect traffic between two Kubernetes clusters each running their
own Consul datacenter.

### Prequisites
The prerequisites listed in the [main gateway docs](/docs/connect/mesh_gateway.html#prerequisites) must be met.

### Step 1 - Get v0.9.0-beta1
Since Gateways are still in Beta, you'll need a specific consul-helm version.

Clone the chart repo.

```sh
$ git clone https://github.com/hashicorp/consul-helm.git
Cloning into 'consul-helm'...
remote: Enumerating objects: 20, done.
remote: Counting objects: 100% (20/20), done.
remote: Compressing objects: 100% (16/16), done.
remote: Total 1578 (delta 9), reused 8 (delta 4), pack-reused 1558
Receiving objects: 100% (1578/1578), 366.38 KiB | 1.09 MiB/s, done.
Resolving deltas: 100% (1175/1175), done.
```

Change into the repo directory.

```
$ cd consul-helm
```

Checkout the v0.9.0-beta1 release.

```
$ git checkout v0.9.0-beta1

Note: checking out 'v0.9.0-beta1'.

You are in 'detached HEAD' state. You can look around, make experimental
...
```

### Step 2 - Configure Your First Datacenter
Add the following config to your helm chart values for dc1.

```yaml
# dc1-values.yaml
meshGateway:
  enabled: true


  # When there are no Connect services running, the gateway fails health checks.
  # As a workaround we will temporarily disable them.
  enableHealthChecks: false

  # Here we're configuring a LoadBalancer service to front our gateway pods.
  service:
    enabled: true
    type: LoadBalancer

# You should also have the config specified in the prerequisites:
global:
  image: consul:1.6.0-beta1
client:
  grpc: true
  extraConfig: |
    {
      "primary_datacenter": "dc1"
    }
server:
  extraConfig: |
    {
      "primary_datacenter": "dc1"
    }
connectInject:
  enabled: true
  centralConfig:
    enabled: true
```

Get your chart name.

```bash
$ helm list
NAME                	REVISION	UPDATED                 	STATUS  	CHART            	APP VERSION	NAMESPACE
incendiary-albatross	3       	Tue Jul  9 19:49:37 2019	DEPLOYED	consul-0.9.0-beta	           	default
```

Then upgrade it.

```bash
$ helm upgrade <your release name> . -f dc1-values.yaml
...
```

You should see the gateway pods starting:

```bash
$ kubectl get pod -l component=mesh-gateway
incendiary-albatross-consul-mesh-gateway-5c9475b97f-h2g9g                 1/1     Running   0          11m
incendiary-albatross-consul-mesh-gateway-5c9475b97f-ptkbd                 1/1     Running   0          11m
```

### Step 3 - Wait for the Load Balancer IP
Now we have our gateway Pods running, but there's no way for the gateway in the
other datacenter to send us requests. We need a Load Balancer in front of our
Pods with an external IP address that the other datacenter can route to.
That's why we configured the helm chart to create a Load Balancer service for us.

We need to wait until our Load Balancer has been allocated an external IP and then
update our Helm configuration. Look for the load balancer's external IP by running:

```bash
$ kubectl get svc -l component=mesh-gateway
```

and wait until you see an external IP has been allocated:

```bash
NAME                               TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)         AGE
incendiary-albatross-consul-mesh-gateway   LoadBalancer   10.35.254.171   32.32.32.32     443:32022/TCP   8m6s
```

Now set the `meshGateway.wanAddress.host` key to the external IP
so remote datacenters can reach our local gateway through the load balancer.

```diff
meshGateway:
  enabled: true
  # Remote gateways will send traffic destined for this datacenter to the WAN address.
+  wanAddress:
+    # We must set useNodeIP to false as it defaults to true.
+    useNodeIP: false
+    host: 32.32.32.32
```

Upgrade your helm release again:

```bash
$ helm upgrade <your release name> . -f dc1-values.yaml
```

### Step 4 - Configure Your Second Datacenter
Now we're ready to start Consul in our second datacenter (`dc2`).

Change your `kubectl` context:

```bash
$ kubectl config get-contexts
...
$ kubectl config use-context your-dc2-context
```

Add the following config to your helm chart values for dc2.

```yaml
# dc2-values.yaml
meshGateway:
  enabled: true
  enableHealthChecks: false
  service:
    enabled: true
    type: LoadBalancer

# You should also have datacenter config and the config specified in the
# prerequisites:
global:
  datacenter: dc2
  image: consul:1.6.0-beta1
client:
  grpc: true
  extraConfig: |
    {
      "primary_datacenter": "dc1"
    }
server:
  extraConfig: |
    {
      "primary_datacenter": "dc1"
    }
connectInject:
  enabled: true
  centralConfig:
    enabled: true
```

And then upgrade your chart:

```bash
$ helm upgrade <your release name> . -f dc2-values.yaml
```

### Step 5 - Wait for the Load Balancer IP
Just like for dc1, we're going to use the load balancer's IP to route to the gateways so we need
to wait for that to be up.

Look for the load balancer's external IP by running:

```bash
$ kubectl get svc -l component=mesh-gateway
```

and waiting until you see an external IP:

```bash
NAME                               TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)         AGE
sanguine-elk-consul-mesh-gateway   LoadBalancer   10.35.254.171   32.32.32.32     443:32022/TCP   8m6s
```

Now set the `meshGateway.wanAddress.host` key to the external IP.

```diff
meshGateway:
  enabled: true
+  wanAddress:
+    useNodeIP: false
+    host: 32.32.32.32
```

Upgrade your helm release again:

```bash
$ helm upgrade <your release name> . -f dc2-values.yaml
```

### Step 6 - Test it out

In dc2 we're going to deploy a simple service that echos back `hello world`.

Create a file `static-server.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: static-server
  annotations:
    # This annotation tells the Connect injector to inject a sidecar
    # proxy that will transparently unencrypt Connect requests.
    "consul.hashicorp.com/connect-inject": "true"
spec:
  containers:
    - name: static-server
      image: hashicorp/http-echo:latest
      args:
        - -text="hello world"
        - -listen=:8080
      ports:
        - containerPort: 8080
          name: http
```

And apply it to dc2:

```bash
$ kubectl apply -f static-server.yaml
pod/static-server created
```

Get the Pod's IP address to prove that we can't route to it directly from the
other datacenter.

```bash
$ kubectl get pod static-server -o=jsonpath='{.status.podIP}'
10.32.5.47%
```

Now switch contexts to dc1:

```bash
$ kubectl config use-context your-dc1-context
Switched to context "your-dc1-context".
```

We're going to deploy a service that will call the `static-server`
service in dc2.

Create a file `static-client.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: static-client
  annotations:
    # This annotation tells the Connect injector to inject a sidecar proxy.
    "consul.hashicorp.com/connect-inject": "true"
    # This annotation configures the sidecar proxy to route localhost:1234
    # traffic to the static-server service running in the dc2 datacenter.
    "consul.hashicorp.com/connect-service-upstreams": "static-server:1234:dc2"
spec:
  containers:
    - name: static-client
      image: tutum/curl:latest
      # Just spin & wait forever, we'll use `kubectl exec` to test.
      command: [ "/bin/sh", "-c", "--" ]
      args: [ "while true; do sleep 30; done;" ]

```

And apply it to **dc1**:

```bash
$ kubectl apply -f static-client.yaml
pod/static-client created
```

Now we're going to `curl` from the service in dc1 directly to the Pod IP from dc1
on port 8080:

```bash
$ curl static-client -c static-client -- curl -s -S --connect-timeout 1 http://10.32.5.47:8080
curl: (28) Connection timed out after 1001 milliseconds
command terminated with exit code 28
```

We should see a connection timeout because the two Pods are on different networks
and should not be able to route to each other.

Now we're going to to use Connect to route us through our gateways to dc2.
We'll use `localhost` because the Connect proxy is listening on localhost
and port `1234` because that's the port we configured it to listen
on with our annotation.

```bash
$ kubectl exec static-client -c static-client -- curl -s http://localhost:1234
"hello world"
```

If you see the `hello world` response then your gateways are working!


When configuring other services, remember to use the correct annotations to target services in other datacenters, i.e.

```yaml
annotations:
  "consul.hashicorp.com/connect-inject": "true"
  "consul.hashicorp.com/connect-service-upstreams": "<remote service name>:<local port>:<remote datacenter name>"
```
