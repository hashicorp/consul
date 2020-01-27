---
name: "Consul-Kubernetes Deployment Guide"
content_length: 14
id: kubernetes-production-deploy
layout: content_layout
products_used:
  - Consul
description: This guide covers the necessary steps to install and configure a new Consul cluster on Kubernetes.
level: Advanced
___


This guide covers the necessary steps to install and configure a new Consul
cluster on Kubernetes, as defined in the [Consul Reference Architecture
guide](/consul/day-1-operations/kubernetes-reference#consul-datacenter-deployed-in-kubernetes).
By the end of this guide, you will be able to identify the installation
prerequisites, customize the Helm chart to fit your environment requirements,
and interact with your new Consul cluster.   

~> You should have the following configured before starting this guide: Helm
installed and configured locally, tiller running in the Kubernetes cluster, and
the Kubernetes CLI configured. 

## Configure Kubernetes Permissions to Deploy Consul

Before deploying Consul, you will need to create a new Kubernetes service
account with the correct permissions and to authenticate it on the command
line. You will need Kubernetes operators permissions to create and modify
policies, deploy services, access the Kubernetes dashboard, create secrets, and
create RBAC objects. You can find documentation for RBAC and service accounts
for the following cloud providers. 
 
- [AKS](https://docs.microsoft.com/en-us/azure/aks/kubernetes-service-principal) 
- [EKS](https://docs.aws.amazon.com/eks/latest/userguide/install-aws-iam-authenticator.html) 
- [GCP](https://console.cloud.google.com/iam-admin/serviceaccounts)

Note, Consul can be deployed on any properly configured Kubernetes cluster in
the cloud or on premises. 

Once you have a service account, you will also need to add a permission to
deploy the helm chart. This is done with the `clusterrolebinding` method. 

```sh
$ kubectl create clusterrolebinding kubernetes-dashboard -n kube-system --clusterrole=cluster-admin --serviceaccount=kube-system:kubernetes-dashboard
```

Finally, you may need to create Kubernetes secrets to store Consul data. You
can reference these secrets in the customized Helm chart values file. 

- If you have purchased Enterprise Consul, the enterprise license file should be
used with the official image,  `hashicorp/consul-enterprise:1.5.0-ent`.  

- Enable
[encryption](https://www.consul.io/docs/agent/encryption.html#gossip-encryption) to secure gossip traffic within the Consul cluster. 


~> Note, depending on your environment, the previous secrets may not be
necessary.  

## Configure Helm Chart 

Now that you have prepared your Kubernetes cluster, you can customize the Helm
chart. 	First, you will need to download the latest official Helm chart.

```sh 
$ git clone https://github.com/hashicorp/consul-helm.git 
```

The `consul-helm` directory will contain a `values.yaml` file with example
parameters. You can update this file to customize your Consul deployment. Below
we detail some of the parameters you should customize and provide an example
file, however you should consider your particular production needs when
configuring your chart. 

### Global Values

The global values will affect all the other parameters in the chart. 

To enable all of the Consul components in the Helm chart, set `enabled` to
`true`. This means servers, clients, Consul DNS, and the Consul UI will be
installed with their defaults. You should also set the following global
parameters based on your specific environment requirements. 

- `image` is the name and tag of the Consul Docker image.  
- `imagek8s` is the name and tag of the Docker image for the consul-k8s binary.  
- `datacenter` the name of your Consul datacenter.  
- `domain` the domain Consul uses for DNS queries. 

For security, set the `bootstrapACLs`  parameter to true. This will enable
Kubernetes to initially setup Consul's [ACL
system](https://www.consul.io/docs/acl/acl-system.html).

Read the Consul Helm chart documentation to review all the [global
parameters](https://www.consul.io/docs/platform/k8s/helm.html#v-global).

### Consul UI

To enable the Consul web UI update the `ui` section to your values file and set
`enabled` to `true`. 

Note, you can also set up a [loadbalancer
resource](https://github.com/hashicorp/demo-consul-101/tree/master/k8s#implement-load-balancer)
or other service type in Kubernetes to make it easier to access the UI.  

### Consul Servers

For production deployments, you will need to deploy [3 or 5 Consul
servers](https://www.consul.io/docs/internals/consensus.html#deployment-table)
for quorum and failure tolerance. For most deployments, 3 servers are adequate.

In the server section set both `replicas` and `bootstrapExpect` to 3. This will
deploy three servers and cause Consul to wait to perform leader election until
all three are healthy. The `resources` will depend on your environment; in the
example at the end of the guide, the resources are set for a large environment. 

#### Affinity

To ensure the Consul servers are placed on different Kubernetes nodes, you will
need to configure affinity. Otherwise, the failure of one Kubernetes node could
cause the loss of multiple Consul servers, and result in quorum loss. By
default, the example `values.yaml` has affinity configured correctly.  

#### Enterprise License

If you have an [Enterprise
license](https://www.hashicorp.com/products/consul/enterprise) you should
reference the Kubernetes secret in the `enterpriseLicense` parameter.

Read the Consul Helm chart documentation to review all the [server
parameters](https://www.consul.io/docs/platform/k8s/helm.html#v-server)

### Consul Clients

A Consul client is deployed on every Kubernetes node, so you do not need to
specify the number of clients for your deployments. You will need to specify
resources and enable gRPC. The resources in the example at the end of this guide
should be
sufficient for most production scenarios since Consul clients are designed for
horizontal scalability. Enabling `grpc` enables the GRPC listener on port 8502
and exposes it to the host. It is required to use Consul Connect.

Read the Consul Helm chart documentation to review all the [client
parameters](https://www.consul.io/docs/platform/k8s/helm.html#v-client)

### Consul Connect Injection Security

Even though you enabled Consul server communication over Connect in the server section, you will also
need to enable `connectInject` by setting `enabled` to `true`. In the
`connectInject` section you will also configure security features. Enabling the
`default` parameter will allow the injector to automatically inject the Connect
sidecar into all pods. If you would prefer to manually annotate which pods to inject, you
can set this to false. Setting the 'aclBindingRuleSelector` parameter to
`serviceaccount.name!=default` ensures that new services do not all receive the
same token if you are only using a default service account. This setting is
only necessary if you have enabled ACLs in the global section.

Read more about the [Connect Inject
parameters](https://www.consul.io/docs/platform/k8s/helm.html#v-connectinject).

## Complete Example

Your finished values file should resemble the following example. For more
complete descriptions of all the available parameters see the `values.yaml`
file  provided with the Helm chart and the [reference
documentation](https://www.consul.io/docs/platform/k8s/helm.html). 

```yaml
# Configure global settings in this section.
global:
  # Enable all the components within this chart by default.
  enabled: true
  # Specify the Consul and consul-k8s  images to use
  image: "consul:1.5.0"
  imagek8s: "hashicorp/consul-k8s:0.8.1"
  domain: consul
  datacenter: primarydc
  # Bootstrap ACLs within Consul. This is highly recommended.
  bootstrapACLs: true
  # Gossip encryption
  gossipEncryption: |
    secretName: "encrypt-key"
    secretKey: "key
# Configure your Consul servers in this section.
server:
  enabled: true
  connect: true
  # Specify three servers that wait till all are healthy to bootstrap the Consul cluster.
  replicas: 3
  bootstrapExpect: 3
  # Specify the resources that servers request for placement. These values will serve a large environment.
  resources: |
    requests:
      memory: "32Gi"
      cpu: "4"
      disk: "50Gi"
    limits:
      memory: "32Gi"
      cpu: "4"
      disk: "50Gi"
  # If using Enterprise, reference the Kubernetes secret that holds your license here
  enterpriseLicense:
    secretName: "consul-license"
    secretKey: "key"
  # Prevent Consul servers from co-location on Kubernetes nodes.
  affinity: |
   podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: {{ template "consul.name" . }}
            release: "{{ .Release.Name }}"
            component: server
      topologyKey: kubernetes.io/hostname
# Configure Consul clients in this section
client:
  enabled: true
  # Specify the resources that clients request for deployment. 
  resources: |
    requests:
      memory: "8Gi"
      cpu: "2"
      disk: "15Gi"
    limits:
      memory: "8Gi"
      cpu: "2"
      disk: "15Gi"
  grpc: true
# Enable and configure the Consul UI.
ui:
  enabled: true
# Configure security for Consul Connect pod injection
connectInject:
  enabled: true
  default: true
  namespaceSelector: "my-namespace"
  aclBindingRuleSelector: “serviceaccount.name!=default” 
```
## Deploy Consul 

Now that you have customized the `values.yml` file, you can deploy Consul with
Helm. This should only take a few minutes. The Consul pods should appear in the
Kubernetes dashboard immediately and you can monitor the deployment process
there.

```sh 
$ helm install ./consul-helm -f values.yaml 
```

To check the deployment process on the command line you can use `kubectl`.

```sh 
$ kubectl get pods 
```

## Summary

In this guide, you configured Consul, using the Helm chart, for a production
environment. This involved ensuring that your cluster had a properly
distributed server cluster, specifying enough resources for your agents,
securing the cluster with ACLs and gossip encryption, and enabling other Consul
functionality including Connect and the Consul UI. 

Now you can interact with your Consul cluster through the UI or CLI. 

If you exposed the UI using a load balancer it will be available at the
`LoadBalancer Ingress` IP address and `Port` that is output from the following
command. Note, you will need to replace _consul server_ with the server name
from your cluster.

```sh 
$ kubectl describe services consul-server 
``` 

To access the Consul CLI, open a terminal session using the Kubernetes CLI.

```sh 
$ kubectl exec <pod name> -it /bin/ash 
```

To learn more about how to interact with your Consul cluster or use it for
service discovery, configuration or segmentation, try one of Learn’s
[Operations or Development tracks](/consul/#advanced). Follow the [Security and
Networking track](/consul/?track=security-networking#security-networking) to
learn more about securing your Consul cluster.


