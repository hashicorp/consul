---
layout: "docs"
page_title: "Deploy Consul with Kubernetes"
sidebar_current: "docs-guides-kuberntes"
description: |-
  Deploy Consul on Kubernetes with the official Helm chart.
---

# Deploy Consul with Kubernetes 

In this guide you will deploy a Consul datacenter with the official Helm chart.
You do not need to update any values in the Helm chart for a basic
installation. However, you can create a values file with parameters to allow
access to the Consul UI. 

~> **Security Warning** This guide is not for production use. By default, the
chart will install an insecure configuration of Consul. Please refer to the
[Kubernetes documentation](https://www.consul.io/docs/platform/k8s/index.html)
to determine how you can secure Consul on Kubernetes in production.
Additionally, it is highly recommended to use a properly secured Kubernetes
cluster or make sure that you understand and enable the recommended security
features. 

To complete this guide successfully, you should have an existing Kubernetes
cluster, and locally configured [Helm](https://helm.sh/docs/using_helm/) and 
[kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/). If you do not have an
existing Kubernetes cluster you can use the [Minikube with Consul guide](https://www.consul.io/docs/guides/minikube.html) to get started
with Consul on Kubernetes. 

## Deploy Consul 

You can deploy a complete Consul datacenter using the official Helm chart. By
default, the chart will install three Consul
servers and client on all Kubernetes nodes. You can review the
[Helm chart
values](https://www.consul.io/docs/platform/k8s/helm.html#configuration-values-)
to learn more about the default settings. 

### Download the Helm Chart

First, you will need to clone the official Helm chart from HashiCorp's Github
repo.

```sh
$ git clone https://github.com/hashicorp/consul-helm.git 
```

You do not need to update the Helm chart before deploying Consul, it comes with
reasonable defaults. Review the [Helm chart
documentation](https://www.consul.io/docs/platform/k8s/helm.html) to learn more
about the chart.

### Helm Install Consul

To deploy Consul you will need to be in the same directory as the chart. 

```sh 
$ cd consul-helm 
```

Now, you can deploy Consul using `helm install`. This will deploy three servers
and agents on all Kubernetes nodes. The process should be quick, less than 5
minutes.  

```sh 
$ helm install ./consul-helm

NAME:   mollified-robin LAST DEPLOYED: Mon Feb 25 15:57:18 2019 NAMESPACE: default STATUS: DEPLOYED
NAME                             READY  STATUS             RESTARTS  AGE
mollified-robin-consul-25r6z     0/1    ContainerCreating  0         0s
mollified-robin-consul-4p6hr     0/1    ContainerCreating  0         0s
mollified-robin-consul-n82j6     0/1    ContainerCreating  0         0s
mollified-robin-consul-server-0  0/1    Pending            0         0s
mollified-robin-consul-server-1  0/1    Pending            0         0s
mollified-robin-consul-server-2  0/1    Pending            0         0s
```

The output above has been reduced for readability. However, you can see that
there are three Consul servers and three Consul clients on this three node
Kubernetes cluster. 

## Access Consul UI

To access the UI you will need to update the `ui` values in the Helm chart.
Alternatively, if you do not wish to upgrade your cluster, you can set up [port
forwarding]
(https://www.consul.io/docs/platform/k8s/run.html#viewing-the-consul-ui) with
`kubectl`. 

### Create Values File

First, create a values file that can be passed on the command line when
upgrading.

```yaml
# values.yaml
global: 
  datacenter: hashidc1 
syncCatalog: 
  enabled: true 
ui: 
  service: 
    type: "LoadBalancer" 
server: 
  affinity: |
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
            matchLabels:
              app: {{ template "consul.name" . }}
              release: "{{ .Release.Name }}"
              component: server
        topologyKey: kubernetes.io/hostname
```

This file renames your datacenter, enables catalog sync, sets up a load
balancer service for the UI, and enables [affinity](https://www.consul.io/docs/platform/k8s/helm.html#v-server-affinity) to allow only one 
Consul pod per Kubernetes node. 
The catalog sync parameters will allow you to see
the Kubernetes services in the Consul UI. 

### Initiate Rolling Upgrade 

Finally, initiate the
[upgrade](https://www.consul.io/docs/platform/k8s/run.html#upgrading-consul-on-kubernetes)
with `helm upgrade` and the `-f` flag that passes in your new values file. This
processes should also be quick, less than a minute.  

```sh
$ helm upgrade consul -f values.yaml 
```

You can now use `kubectl get services` to discover the external IP of your Consul UI.

```sh 
$ kubectl get services 
NAME                            TYPE           CLUSTER-IP     EXTERNAL-IP             PORT(S)        AGE 
consul                          ExternalName   <none>         consul.service.consul   <none>         11d 
kubernetes                      ClusterIP      122.16.14.1    <none>                  443/TCP        137d
mollified-robin-consul-dns      ClusterIP      122.16.14.25   <none>                  53/TCP,53/UDP  13d
mollified-robin-consul-server   ClusterIP      None           <none>                  8500/TCP       13d
mollified-robin-consul-ui       LoadBalancer   122.16.31.395  36.276.67.195           80:32718/TCP   13d
```

Additionally, you can use `kubectl get pods` to view the new catalog sync
process. The [catalog sync](https://www.consul.io/docs/platform/k8s/helm.html#v-synccatalog) process will sync 
Consul and Kubernetes services bidirectionally by 
default.

```
$ kubectl get pods
NAME                                                 READY   STATUS      RESTARTS   AGE
mollified-robin-consul-d8mnp                          1/1     Running     0         15d
mollified-robin-consul-p4m89                          1/1     Running     0         15d
mollified-robin-consul-qclqc                          1/1     Running     0         15d
mollified-robin-consul-server-0                       1/1     Running     0         15d
mollified-robin-consul-server-1                       1/1     Running     0         15d
mollified-robin-consul-server-2                       1/1     Running     0         15d
mollified-robin-consul-sync-catalog-f75cd5846-wjfdk   1/1     Running     0         13d
```

The service should have `consul-ui` appended to the deployment name. Note, you
do not need to specify a port when accessing the dashboard. 

## Access Consul 

In addition to accessing Consul with the UI, you can manage Consul with the
HTTP API or by directly connecting to the pod with `kubectl`. 

### Kubectl

To access the pod and data directory you can exec into the pod with `kubectl` to start a shell session.

```sh 
$ kubectl exec -it mollified-robin-consul-server-0 /bin/sh 
```

This will allow you to navigate the file system and run Consul CLI commands on
the pod. For example you can view the Consul members. 

```sh 
$ consul members 
Node                                   Address           Status  Type    Build  Protocol  DC        Segment 
mollified-robin-consul-server-0        172.20.2.18:8301  alive   server  1.4.2  2         hashidc1  <all>
mollified-robin-consul-server-1        172.20.0.21:8301  alive   server  1.4.2  2         hashidc1  <all> 
mollified-robin-consul-server-2        172.20.1.18:8301  alive   server  1.4.2  2         hashidc1  <all>
gke-tier-2-cluster-default-pool-leri5  172.20.1.17:8301  alive   client  1.4.2  2         hashidc1  <default>
gke-tier-2-cluster-default-pool-gnv4   172.20.2.17:8301  alive   client  1.4.2  2         hashidc1  <default>
gke-tier-2-cluster-default-pool-zrr0   172.20.0.20:8301  alive   client  1.4.2  2         hashidc1  <default>
```

### Consul HTTP API

You can use the Consul HTTP API by communicating to the local agent running on
the Kubernetes node. You can read the
[documentation](https://www.consul.io/docs/platform/k8s/run.html#accessing-the-consul-http-api)
if you are interested in learning more about using the Consul HTTP API with Kubernetes.

## Summary

In this guide, you deployed a Consul datacenter in Kubernetes using the
official Helm chart. You also configured access to the Consul UI. To learn more
about deploying applications that can use Consul's service discovery and
Connect, read the example in the [Minikube with Consul
guide](https://www.consul.io/docs/guides/minikube.html#step-2-deploy-custom-applications).

