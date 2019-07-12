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

### Prequisites
The following conditions must be satisfied for gateways to work between Consul datacenters
in Kubernetes:

1. Both clusters are running Consul **v1.6.0-beta1** since gateways are a beta feature.
    This can be set via the helm chart:
    
    ```yaml
    global:
      image: consul:1.6.0-beta1
    ```
1. Each cluster has a unique datacenter name, ex. `dc1` and `dc2`.
    This can be set via the helm chart:
    
    ```yaml
    global:
      datacenter: dc2 # use dc1 in your other datacenter.
    ``` 
    
1. The Consul datacenters are joined. This will require that all of the Consul server agents
   in both datacenters have access to each other, and that they are
   configured to join with the other datacenter. See our [Basic Datacenter Federation Guide](https://learn.hashicorp.com/consul/security-networking/datacenters) to learn how to join servers in different datacenters to each other with WAN gossip.
1. The `primary_datacenter` setting is set to the same value in both Consul datacenters. This is required
   for Connect requests to work across datacenters. This can be set via the helm chart:
   
    ```yaml
    client:
      extraConfig: |
        {
          "primary_datacenter": "dc1"
        }
    server:
      extraConfig: |
        {
          "primary_datacenter": "dc1"
        }
    ```
1. If using Kubernetes, Connect injection must be enabled. This can be set via the helm chart:

    ```yaml
    connectInject:
      enabled: true
    ```
1. gRPC must be enabled. This can be set via the helm chart:

    ```yaml
    client:
      grpc: true
    ```
1. If setting a global default [gateway mode](/docs/connect/mesh_gateway.html#modes-of-operation),
   central config must be enabled for all agents so that they actually use the default mode.
   This can be set via the helm chart:

    ```yaml
    connectInject:
      centralConfig:
       enabled: true
    ```

## Guide to Using Mesh Gateways To Connect Two Kubernetes Clusters
This guide shows how to configure the consul-helm chart to set up gateways
that route Connect traffic between two Kubernetes clusters each running their
own Consul datacenter.

### Prequisites
The prerequisites [specified above](/docs/platform/k8s/mesh-gateways.html#prequisites)
must be met.

### Step 1 - Get v0.9.0-beta1
Since Gateways are still in Beta, you'll need a specific consul-helm version.

```sh
# Clone the chart repo
$ git clone https://github.com/hashicorp/consul-helm.git
$ cd consul-helm

# Checkout the v0.9.0-beta1 release.
$ git checkout v0.9.0-beta1

#  You should see:
#  Note: checking out 'v0.9.0-beta1'.
#
#  You are in 'detached HEAD' state. You can look around, make experimental
#  ...
```

### Step 2 - Configure Your First Datacenter
Add the following config to your helm chart values for dc1.

```yaml
# dc1-values.yaml
meshGateway:
  enabled: true

  # Remote gateways will send traffic destined for this datacenter to the WAN address.
  # traffic to.
  wanAddress:
    # For now we're setting this to the node IP but in the next step we'll set
    # it to our Load Balancer's IP.
    useNodeIP: true

  # When there are no Connect services running, the gateway fails health checks.
  # As a workaround we will temporarily disable them.
  enableHealthChecks: false

  # Here we're configuring a LoadBalancer service to front our gateway pods.
  service:
    enabled: true
    type: LoadBalancer

# You should also have the config specified in the prerequisites.
```

And then upgrade your chart:

```bash
# Get your chart name.
$ helm list

# Then upgrade it.
$ helm upgrade <your release name> . -f dc1-values.yaml
```

You should see the gateway pods starting:

```bash
$ kubectl get pod -l component=mesh-gateway
sanguine-elk-consul-mesh-gateway-5c9475b97f-h2g9g                 1/1     Running   0          11m
sanguine-elk-consul-mesh-gateway-5c9475b97f-ptkbd                 1/1     Running   0          11m
```

### Step 3 - Wait for the Load Balancer IP
We're going to use the load balancer's IP to route to the gateways so we need
to wait for that to be up.

Run

```bash
kubectl get svc -l component=mesh-gateway
```
and wait until you see an external IP:

```bash
NAME                               TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)         AGE
sanguine-elk-consul-mesh-gateway   LoadBalancer   10.35.254.171   32.32.32.32     443:32022/TCP   8m6s
```

Now change the `meshGateway.wanAddress.host` key to the external IP.
This will tell remote datacenters to route gateway traffic to the load balancer.

```diff
meshGateway:
  enabled: true
  wanAddress:
-    useNodeIP: true
+    useNodeIP: false
+    host: 32.32.32.32
```

Upgrade your helm release again:

```bash
helm upgrade <your release name> . -f dc1-values.yaml
```

### Step 4 - Configure Your Second Datacenter
Now we're ready to start Consul in our second datacenter (`dc2`).

Change your `kubectl` context:

```bash
kubectl config get-contexts
...
kubectl config use-context your-dc2-context
```

Add the following config to your helm chart values for dc2.

```yaml
# dc2-values.yaml
meshGateway:
  enabled: true
  wanAddress:
    useNodeIP: true
  enableHealthChecks: false
  service:
    enabled: true
    type: LoadBalancer

# You should also have datacenter config:
global:
  datacenter: dc2
    
# You should also have the config specified in the prerequisites.
```

And then upgrade your chart:

```bash
helm upgrade <your release name> . -f dc2-values.yaml
```

### Step 5 - Wait for the Load Balancer IP
We're going to use the load balancer's IP to route to the gateways so we need
to wait for that to be up.

Run

```bash
kubectl get svc -l component=mesh-gateway
```
and wait until you see an external IP:

```bash
NAME                               TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)         AGE
sanguine-elk-consul-mesh-gateway   LoadBalancer   10.35.254.171   32.32.32.32     443:32022/TCP   8m6s
```

Now change the `meshGateway.wanAddress.host` key to the IP:

```diff
meshGateway:
  enabled: true
  wanAddress:
-    useNodeIP: true
+    useNodeIP: false
+    host: 32.32.32.32
```

Upgrade your helm release again:

```bash
helm upgrade <your release name> . -f dc2-values.yaml
```

### Step 6 - Ensure Each Datacenter Can Access the Load Balancers
Ensure that every Pod in dc1 can make requests to the load balancer IP from dc2
and vice versa.

You won't be able to make a successful request but it should at least connect:

```bash
$ kubectl exec sanguine-elk-consul-server-0 -- curl https://32.32.32.32
curl: (35) Unknown SSL protocol error in connection to 32.32.32.32:443
```

### Step 7 - Test it out

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
kubectl apply -f static-server.yaml
```

Now switch contexts to dc1 and deploy a service that will call the `static-server`
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
kubectl apply -f static-client.yaml
```

Now we're going to `curl` from the service in dc1 through our gateways to dc2:

```bash
kubectl exec static-client -c static-client -- curl -s http://localhost:1234
# You should see the response:
"hello world"
```

Now your gateways are working! Remember to use the correct annotations to
target services in other datacenters, i.e.

```yaml
annotations:
  "consul.hashicorp.com/connect-inject": "true"
  "consul.hashicorp.com/connect-service-upstreams": "<remote service name>:<localhost port>:<remote datacenter name>"
```
