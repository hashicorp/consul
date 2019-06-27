---
layout: "docs"
page_title: "Layer 7 Observability with Kubernetes and Consul Connect"
sidebar_current: "docs-guides-kubernetes-observability"
description: |-
  Collect and visualize layer 7 metrics from services in your Kubernetes cluster
  using Consul  Connect, Prometheus, and Grafana.
---

A service mesh is made up of proxies deployed locally alongside each service
instance, which control network traffic between their local instance and other
services on the network. These proxies "see" all the traffic that runs through
them, and in addition to securing that traffic, they can also collect data about
it. Starting with version 1.5, Consul Connect is able to configure Envoy proxies
to collect layer 7 metrics including HTTP status codes and request latency, along
with many others, and export those to monitoring tools like Prometheus.

In this guide, you will deploy a basic metrics collection and visualization
pipeline on a Kubernetes cluster using the official Helm charts for Consul,
Prometheus, and Grafana. This pipeline will collect and display metrics from a
demo application.

-> **Tip:** While this guide shows you how to deploy a metrics pipeline on
Kubernetes, all the technologies the guide uses are platform agnostic;
Kubernetes is not necessary to collect and visualize layer 7 metrics with Consul
Connect.

Learning Objectives:

- Configure Consul Connect with metrics using Helm
- Install Prometheus and Grafana using Helm
- Install and start the demo application
- Collect metrics

## Prerequisites

If you already have a Kubernetes cluster with Helm and kubectl up and running,
you can start on the demo right away. If not, set up a Kubernetes cluster using
your favorite method that supports persistent volume claims, or install and
start [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/). If
you do use Minikube, you may want to start it with a little bit of extra memory.

```bash
$ minikube start --memory 4096
```

You will also need to install
[kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl),
and both install and initialize
[Helm](https://helm.sh/docs/using_helm/#installing-helm) by following their
official instructions.

If you already had Helm installed, check that you have up
to date versions of the Grafana, Prometheus, and Consul charts. You can update
all your charts to the latest versions by running `helm repo update`.

Clone the GitHub repository that contains the configuration files you'll use
while following this guide, and change directories into it. We'll refer to this
directory as your working directory, and you'll run the rest of the commands in
this guide from inside it.

```bash
$ git clone https://github.com/hashicorp/consul-k8s-l7-obs-guide.git

$ cd consul-k8s-l7-obs-guide
```

## Deploy Consul Connect Using Helm

Once you have set up the prerequisites, you're ready to install Consul. Start by
cloning the official Consul Helm chart into your working directory.

```bash
$ git clone https://github.com/hashicorp/consul-helm.git
```

Open the file in your working directory called `consul-values.yaml`. This file
will configure the Consul Helm chart to:

- specify a name for your Consul datacenter
- enable the Consul web UI
- enable secure communication between pods with Connect
- configure the Consul settings necessary for layer 7 metrics collection
- specify that this Consul cluster should run one server
- enable metrics collection on servers and agents so that you can monitor the
  Consul cluster itself

You can override many of the values in Consul's values file using annotations on
specific services. For example, later in the guide you will override the
centralized configuration of `defaultProtocol`.

```yaml
# name your datacenter
global:
  datacenter: dc1

server:
  # use 1 server
  replicas: 1
  bootstrapExpect: 1
  disruptionBudget:
    enabled: true
    maxUnavailable: 0

client:
  enabled: true
  # enable grpc on your client to support consul connect
  grpc: true

ui:
  enabled: true

connectInject:
  enabled: true
  # inject an envoy sidecar into every new pod,
  # except for those with annotations that prevent injection
  default: true
  # these settings enable L7 metrics collection and are new in 1.5
  centralConfig:
    enabled: true
    # set the default protocol (can be overwritten with annotations)
    defaultProtocol: "http"
    # tell envoy where  to send metrics
    proxyDefaults: |
      {
      "envoy_dogstatsd_url": "udp://127.0.0.1:9125"
      }
```

!> **Warning:** By default, the chart will install an insecure configuration of
Consul. This provides a less complicated out-of-box experience for new users but
is not appropriate for a production setup. Make sure that your Kubernetes
cluster is properly secured to prevent unwanted access to Consul, or that you
understand and enable the
[recommended Consul security features](/docs/internals/security.html).
Currently, some of these features are not supported in the Helm chart and
require additional manual configuration.

Now install Consul in your Kubernetes cluster and give Kubernetes a name for
your Consul installation. The output will be a list of all the Kubernetes
resources created (abbreviated in the code snippet).

```bash
$ helm install -f consul-values.yaml --name l7-guide ./consul-helm
NAME:   consul
LAST DEPLOYED: Wed May  1 16:02:40 2019
NAMESPACE: default
STATUS: DEPLOYED

RESOURCES:
```

Check that Consul is running in your Kubernetes cluster via the Kubernetes
dashboard or CLI. If you are using Minikube, the below command will run in your
current terminal window and automatically open the dashboard in your browser.

```bash
$ minikube dashboard
```

Open a new terminal tab to let the dashboard run in the current one, and change
directories back into `consul-k8s-l7-obs-guide`. Next, forward the port for the
Consul UI to localhost:8500 and navigate to it in your browser. Once you run the
below command it will continue to run in your current terminal window for as
long as it is forwarding the port.

```bash
$ kubectl port-forward l7-guide-consul-server-0 8500:8500
Forwarding from 127.0.0.1:8500 -> 8500
Forwarding from [::1]:8500 -> 8500
Handling connection for 8500
```

Let the consul dashboard port forwarding run and open a new terminal tab to the
`consul-k8s-l7-obs-guide` directory.

## Deploy the Metrics Pipeline

In this guide, you will use Prometheus and Grafana to collect and visualize
metrics. Consul Connect can integrate with a variety of other metrics tooling as
well.

### Deploy Prometheus with Helm

You'll follow a similar process as you did with Consul to install Prometheus via
Helm. First, open the file named `prometheus-values.yaml` that configures the
Prometheus Helm chart.

The file specifies how often Prometheus should scrape for metrics, and which
endpoints it should scrape from. By default, Prometheus scrapes all the
endpoints that Kubernetes knows about, even if those endpoints don't expose
Prometheus metrics. To prevent Prometheus from scraping these endpoints
unnecessarily, the values file includes some relabel configurations.

Install the official Prometheus Helm chart using the values in
`prometheus-values.yaml`.

```bash
$ helm install -f prometheus-values.yaml --name prometheus stable/prometheus
NAME:   prometheus
LAST DEPLOYED: Wed May  1 16:09:48 2019
NAMESPACE: default
STATUS: DEPLOYED

RESOURCES:
```

The output above has been abbreviated; you will see all the Kubernetes resources
that the Helm chart created. Once Prometheus has come up, you should be able to
see your new services on the Minikube dashboard and in the Consul UI. This
might take a short while.

### Deploy Grafana with Helm

Installing Grafana will follow a similar process. Open and look through the file
named `grafana-values.yaml`. It configures Grafana to use Prometheus as its
datasource.

Use the official Helm chart to install Grafana with your values file.

```bash
$ helm install -f grafana-values.yaml --name grafana stable/grafana
NAME:   grafana
LAST DEPLOYED: Wed May  1 16:57:11 2019
NAMESPACE: default
STATUS: DEPLOYED

RESOURCES:

...

NOTES:
1. Get your 'admin' user password by running:

   kubectl get secret --namespace default grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo

2. The Grafana server can be accessed via port 80 on the following DNS name from within your cluster:

   grafana.default.svc.cluster.local

   Get the Grafana URL to visit by running these commands in the same shell:

     export POD_NAME=$(kubectl get pods --namespace default -l "app=grafana,release=grafana" -o jsonpath="{.items[0].metadata.name}")
     kubectl --namespace default port-forward $POD_NAME 3000

3. Login with the password from step 1 and the username: admin
```

Again, the above output has been abbreviated. At the bottom of your terminal
output are shell-specific instructions to access your Grafana UI and log in,
displayed as a numbered list. Accessing Grafana involves:

1. Getting the secret that serves as your Grafana password
1. Forwarding the Grafana UI to localhost:3000, which will not succeed until
   Grafana is running
1. Visiting the UI and logging in

Once you have logged into the Grafana UI, hover over the dashboards icon (four
squares in the left hand menu) and then click the "manage" option. This will
take you to a page that gives you some choices about how to upload Grafana
dashboards. Click the black "Import" button on the right hand side of the
screen.

![Add a dashboard using the Grafana GUI](/assets/images/consul-grafana-add-dash.png)

Open the file called `overview-dashboard.json` and copy the contents into the
json window of the Grafana UI. Click through the rest of the options, and you
will end up with a blank dashboard, waiting for data to display.

### Deploy a Demo Application on Kubernetes

Now that your monitoring pipeline is set up, deploy a demo application that will
generate data. We will be using Emojify, an application that recognizes faces in
an image and pastes emojis over them. The application consists of a few
different services and automatically generates traffic and HTTP error codes.

All the files defining Emojify are in the `app` directory. Open `app/cache.yml`
and take a look at the file. Most of services that make up Emojify communicate
over HTTP, but the cache service uses gRPC. In the annotations section of the
file you'll see where `consul.hashicorp.com/connect-service-protocol` specifies
gRPC, overriding the `defaultProtocol` of  HTTP that we centrally configured in
Consul's value file.

At the bottom of each file defining part of the Emojify app, notice the block
defining a `prometheus-statsd` pod. These pods translate the metrics that Envoy
exposes to a format that Prometheus can scrape. They won't be necessary anymore
once Consul Connect becomes compatible with Envoy 1.10. Apply the configuration
to deploy Emojify into your cluster.

```bash
$ kubectl apply -f app
```

Emojify will take a little while to deploy. Once it's running you can check that
it's healthy by taking a look at your Kubernetes dashboard or Consul UI. Next,
visit the Emojify UI. This will be located at the IP address of the host where
the ingress server is running, at port 30000. If you're using Minikube you can
find the UI with the following command.

```bash
$ minikube service emojify-ingress --url
http://192.168.99.106:30000
```

Test the application by emojifying a picture. You can do this by pasting the
following URL into the URL bar and clicking the submit button. (We provide a
demo URL because Emojify can be picky about processing some image URLs if they
don't link directly to the actual picture.)

`https://emojify.today/pictures/1.jpg`

Now that you know the application is working, start generating automatic load so
that you will have some interesting metrics to look at.

```bash
$ kubectl apply -f traffic.yaml
```

## Collect Application Metrics

Envoy exposes a huge number of
[metrics](https://www.envoyproxy.io/docs/envoy/v1.10.0/operations/stats_overview),
but you will probably only want to monitor or alert on a subset of them. Which
metrics are important to monitor will depend on your application. For this
getting-started guide we have preconfigured an Emojify-specific Grafana
dashboard with a couple of basic metrics, but you should systematically consider
what others you will need to collect as you move from testing into production.

### Review Dashboard Metrics

Now that you have metrics flowing through your pipeline, navigate back to your
Grafana dashboard at `localhost:3000`. The top row of the dashboard displays
general metrics about the Emojify application as a whole, including request and
error rates. Although changes in these metrics can reflect application health
issues once you understand their baseline levels, they don't provide enough
information to diagnose specific issues.

The following rows of the dashboard report on some of the specific services that
make up the emojify application: the website, API, and cache services. The
website and API services show request count and response time, while the cache
reports on request count and methods.

## Clean up

If you've been using Minikube, you can tear down your environment by running
`minikube delete`.

If you want to get rid of the configurations files and Consul Helm chart,
recursively remove the `consul-k8s-l7-obs-guide` directory.
`

```bash
$ cd ..

$ rm -rf consul-k8s-l7-obs-guide
```

## Summary

In this guide, you set up layer 7 metrics collection and visualization in a
Minikube cluster using Consul Connect, Prometheus, and Grafana, all deployed via
Helm charts. Because all of these programs can run outside of Kubernetes, you
can set this pipeline up in any environment or collect metrics from workloads
running on mixed infrastructure.

To learn more about the configuration options in Consul that enable layer 7
metrics collection with or without Kubernetes, refer to [our
documentation](/docs/connect/proxies/envoy.html). For more information on
centrally configuring Consul, take a look at the [centralized configuration
documentation](/docs/agent/config_entries.html).
