---
layout: "docs"
page_title: "Helm - Kubernetes"
sidebar_current: "docs-platform-k8s-helm"
description: |-
  The Consul Helm chart is the recommended way to install and configure Consul on Kubernetes. In addition to running Consul itself, the Helm chart is the primary method for installing and configuring Consul integrations with Kubernetes such as catalog syncing, Connect injection, and more.
---

# Helm Chart

The [Consul Helm chart](https://github.com/hashicorp/consul-helm)
is the recommended way to install and configure Consul on Kubernetes.
In addition to running Consul itself, the Helm chart is the primary
method for installing and configuring Consul integrations with
Kubernetes such as catalog syncing, Connect injection, and more.

This page assumes general knowledge of [Helm](https://helm.sh/) and
how to use it. Using Helm to install Consul will require that Helm is
properly installed and configured with your Kubernetes cluster.

-> **Important:** The Helm chart is new and
may still change significantly over time. Please always run Helm with
`--dry-run` before any install or upgrade to verify changes.

## Using the Helm Chart

To use the Helm chart, you must download or clone the
[consul-helm GitHub repository](https://github.com/hashicorp/consul-helm)
and run Helm against the directory. We plan to transition to using a real
Helm repository soon. When running Helm, we highly recommend you always
checkout a specific tagged release of the chart to avoid any
instabilities from master.

Prior to this, you must have Helm installed and configured both in your
Kubernetes cluster and locally on your machine. The steps to do this are
out of the scope of this document, please read the
[Helm documentation](https://helm.sh/) for more information.

Example chart usage:

```sh
# Clone the chart repo
$ git clone https://github.com/hashicorp/consul-helm.git
$ cd consul-helm

# Checkout a tagged version
$ git checkout v0.1.0

# Run Helm
$ helm install --dry-run ./
```

~> **Warning:** By default, the chart will install _everything_: a
Consul server cluster, client agents on all nodes, feature components, etc.
This provides a nice out-of-box experience for new users, but may not be
appropriate for a production setup. Consider setting the `global.enabled`
value to `false` and opt-in to the various components.

## Configuration (Values)

The chart is highly customizable using
[Helm configuration values](https://docs.helm.sh/using_helm/#customizing-the-chart-before-installing).
Each value has a sane default tuned for an optimal getting started experience
with Consul. Before going into production, please review the parameters below
and consider if they're appropriate for your deployment.

* <a name="v-global" href="#v-global">`global`</a> - These values are global values that affect multiple components of the chart.

  - <a name="v-global-enabled" href="#v-global-enabled">`enabled`</a> (`boolean: true`) -
  The master enabled/disabled configuration. If this is true, most components
  will be installed by default. If this is false, no components will be installed
  by default and manually opt-in is required, such as by setting
  <a href="#v-">`server.enabled`</a> to true.


  - <a name="v-global-domain" href="#v-global-domain">`domain`</a> (`string: "consul"`) -
  The domain Consul uses for DNS queries. This is used to configure agents both
  for DNS listening but also to know what domain to join to join the cluster.
  This should be consistent throughout the chart, but can be overridden
  per-component as well.

  - <a name="v-global-image" href="#v-global-image">`image`</a> (`string: "consul:latest"`) -
  The name of the Docker image (including any tag) for the containers running
  Consul agents. **This should be pinned to a specific version when running
  in production.** Otherwise, other changes to the chart may inadvertently
  upgrade your Consul version.

  - <a name="v-global-imagek8s" href="#v-global-imagek8s">`imageK8S`</a> (`string: "hashicorp/consul-k8s:latest"`) -
  The name of the Docker image (including any tag) for the
  [consul-k8s](https://github.com/hashicorp/consul-k8s) binary. This is
  used by components such as catalog sync. **This should be pinned to a specific
  version when running in production.** Otherwise, other changes to the chart may
  inadvertently upgrade the version.

  - <a name="v-global-datacenter" href="#v-global-datacenter">`datacenter`</a> (`string: "dc1"`) -
  The name of the datacenter that the agent cluster should register as.
  This may not be changed once the cluster is bootstrapped and running,
  since Consul doesn't yet support an automatic way to change this value.


* <a name="v-server" href="#v-server">`server`</a> - Values that configure
  running a Consul server cluster within Kubernetes.

  - <a name="v-server-enabled" href="#v-server-enabled">`enabled`</a> (`boolean: global.enabled`) -
  If true, the chart will install all the resources necessary for a Consul
  server cluster. If you're running Consul externally and want agents within
  Kubernetes to join that cluster, this should probably be false.

  - <a name="v-server-image" href="#v-server-image">`image`</a> (`string: global.image`) -
  The name of the Docker image (including any tag) for the containers running
  Consul server agents.

  - <a name="v-server-replicas" href="#v-server-replicas">`replicas`</a> (`integer: 3`) -
  The number of server agents to run. This determines the fault tolerance
  of the cluster. Please see the [deployment table](/docs/internals/consensus.html#deployment-table)
  for more information.

  - <a name="v-server-bootstrapexpect" href="#v-server-bootstrapexpect">`bootstrapExpect`</a> (`integer: 3`) -
  For new clusters, this is the number of servers to wait for before
  performing the initial leader election and bootstrap of the cluster. This
  must be less than or equal to `server.replicas`. This value is only
  used when bootstrapping new clusters, it has no effect during ongoing cluster
  maintenance.

  - <a name="v-server-storage" href="#v-server-storage">`storage`</a> (`string: 10Gi`) -
  This defines the disk size for configuring the servers' StatefulSet storage. For dynamically
  provisioned storage classes, this is the desired size. For manually defined persistent
  volumes, this should be set to the disk size of the attached volume.

  - <a name="v-server-storageclass" href="#v-server-storageclass">`storageClass`</a> (`string: null`) -
  The StorageClass to use for the servers' StatefulSet storage. It must be able to be dynamically
  provisioned if you want the storage to be automatically created. For example, to use
  [Local](https://kubernetes.io/docs/concepts/storage/storage-classes/#local)
  storage classes, the PersistentVolumeClaims would need to be manually created. A `null` value will
  use the Kubernetes cluster's default StorageClass. If a default StorageClass does not exist,
  you will need to create one.

  - <a name="v-server-connect" href="#v-server-connect">`connect`</a> (`boolean: true`) -
  This will enable/disable [Connect](/docs/connect/index.html). Setting this
  to true _will not_ automatically secure pod communication, this setting will
  only enable usage of the feature. Consul will automatically initialize a new
  CA and set of certificates. Additional Connect settings can be configured
  by setting the `server.extraConfig` value.

  - <a name="v-server-resources" href="#v-server-resources">`resources`</a> (`object: {}`) -
  The resource requests (CPU, memory, etc.) for each of the server agents.
  This should be an object mapping directly to a Kubernetes
  [ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#resourcerequirements-v1-core) object. If this isn't specified, then the pods
  won't request any specific amount of resources. **Setting this is highly
  recommended.**

  - <a name="v-server-updatepartition" href="#v-server-updatepartition">`updatePartition`</a> (`integer: 0`) -
  This value is used to carefully control a rolling update of Consul server
  agents. This value specifies the
  [partition](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#partitions)
  for performing a rolling update. Please read the linked Kubernetes
  documentation for more information.

  - <a name="v-server-disruptionbudget" href="#v-server-disruptionbudget">`disruptionBudget`</a> -
  This configures the
  [PodDisruptionBudget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/)
  for the server cluster.

      * <a name="v-server-disruptionbudget-enabled" href="#v-server-disruptionbudget-enabled">`enabled`</a> (`boolean: true`) -
      This will enable/disable registering a PodDisruptionBudget for
      the server cluster. If this is enabled, it will only register the
      budget so long as the server cluster is enabled.

      * <a name="v-server-disruptionbudget-maxunavailable" href="#v-server-disruptionbudget-maxunavailable">`maxUnavailable`</a> (`integer: null`) -
      The maximum number of unavailable pods. By default, this will be automatically
      computed based on the `server.replicas` value to be `(n/2)-1`. If you need to set
      this to `0`, you will need to add a `--set 'server.disruptionBudget.maxUnavailable=0'`
      flag to the helm chart installation command because of a limitation in the Helm
      templating language.

  - <a name="v-server-extraconfig" href="#v-server-extraconfig">`extraConfig`</a> (`string: "{}"`) -
  A raw string of extra JSON or HCL configuration for Consul servers. This
  will be saved as-is into a ConfigMap that is read by the Consul server agents.
  This can be used to add additional configuration that isn't directly exposed
  by the chart.

  - <a name="v-server-extravolumes" href="#v-server-extravolumes">`extraVolumes`</a> (`array: []`) -
  A list of extra volumes to mount for server agents. This is useful for bringing
  in extra data that can be referenced by other configurations at a well known
  path, such as TLS certificates or Gossip encryption keys.
  The value of this should be a list of objects. Each object supports the following keys:

      * <a name="v-server-extravolumes-type" href="#v-server-extravolumes-type">`type`</a> (`string: required`) -
      Type of the volume, must be one of "configMap" or "secret". Case sensitive.

      * <a name="v-server-extravolumes-name" href="#v-server-extravolumes-name">`name`</a> (`string: required`) -
      Name of the configMap or secret to be mounted. This also controls the path
      that it is mounted to. The volume will be mounted to `/config/userconfig/<name>`.

      * <a name="v-server-extravolumes-load" href="#v-server-extravolumes-load">`load`</a> (`boolean: false`) -
      If true, then the agent will be configured to automatically load HCL/JSON
      configuration files from this volume with `-config-dir`. This defaults
      to false.

  - <a name="v-server-affinity" href="#v-server-affinity">`affinity`</a> (`string`) -
  This value defines the [affinity](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity)
  for server pods. It defaults to allowing only a single pod on each node,
  which minimizes risk of the cluster becoming unusable if a node is lost.
  If you need to run more pods per node (for example, testing on Minikube),
  set this value to `null`.

* <a name="v-client" href="#v-client">`client`</a> - Values that configure
  running a Consul client agent on Kubernetes nodes.

  - <a name="v-client-enabled" href="#v-client-enabled">`enabled`</a> (`boolean: global.enabled`) -
  If true, the chart will install all the resources necessary for a Consul
  client on every Kubernetes node. This _does not_ require `server.enabled`,
  since the agents can be configured to join an external cluster.

  - <a name="v-client-image" href="#v-client-image">`image`</a> (`string: global.image`) -
  The name of the Docker image (including any tag) for the containers running
  Consul client agents.

  - <a name="v-client-grpc" href="#v-client-grpc">`grpc`</a> (`boolean: false`) -
  If true, agents will enable their GRPC listener on port 8502 and expose
  it to the host. This will use slightly more resources, but is required for
  [Connect](/docs/platform/k8s/connect.html).

  - <a name="v-client-extraconfig" href="#v-client-extraconfig">`extraConfig`</a> (`string: "{}"`) -
  A raw string of extra JSON or HCL configuration for Consul clients. This
  will be saved as-is into a ConfigMap that is read by the Consul agents.
  This can be used to add additional configuration that isn't directly exposed
  by the chart.

  - <a name="v-client-extravolumes" href="#v-client-extravolumes">`extraVolumes`</a> (`array: []`) -
  A list of extra volumes to mount for client agents. This is useful for bringing
  in extra data that can be referenced by other configurations at a well known
  path, such as TLS certificates or Gossip encryption keys.
  The value of this should be a list of objects. Each object supports the following keys:

      * <a name="v-client-extravolumes-type" href="#v-client-extravolumes-type">`type`</a> (`string: required`) -
      Type of the volume, must be one of "configMap" or "secret". Case sensitive.

      * <a name="v-client-extravolumes-name" href="#v-client-extravolumes-name">`name`</a> (`string: required`) -
      Name of the configMap or secret to be mounted. This also controls the path
      that it is mounted to. The volume will be mounted to `/config/userconfig/<name>`.

      * <a name="v-client-extravolumes-load" href="#v-client-extravolumes-load">`load`</a> (`boolean: false`) -
      If true, then the agent will be configured to automatically load HCL/JSON
      configuration files from this volume with `-config-dir`. This defaults
      to false.

  - <a name="v-client-join" href="#v-client-join">`join`</a> (`array<string>: null`) -
  A list of valid [`-retry-join` values](/docs/agent/options.html#retry-join).
  If this is `null` (default),
  then the clients will attempt to automatically join the server cluster
  running within Kubernetes. This means that with `server.enabled` set to true,
  clients will automatically join that cluster. If `server.enabled` is not
  true, then a value must be specified so the clients can join a valid cluster.

  - <a name="v-client-resources" href="#v-client-resources">`resources`</a> (`object: {}`) -
  The resource requests (CPU, memory, etc.) for each of the client agents.
  This should be an object mapping directly to a Kubernetes
  [ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#resourcerequirements-v1-core) object. If this isn't specified, then the pods
  won't request any specific amount of resources.

* <a name="v-dns" href="#v-dns">`dns`</a> - Values that configure the Consul DNS service.

  - <a name="v-dns-enabled" href="#v-dns-enabled">`enabled`</a> (`boolean: global.enabled`) -
  If true, a `consul-dns` service will be created that exposes port 53 for
  TCP and UDP to the running Consul agents (servers and clients). This can
  then be used to [configure kube-dns](/docs/platform/k8s/dns.html). The Helm
  chart _does not_ automatically configure kube-dns.

* <a name="v-synccatalog" href="#v-synccatalog">`syncCatalog`</a> - Values that
  configure running the [service sync](/docs/platform/k8s/service-sync.html)
  process.

  - <a name="v-synccatalog-enabled" href="#v-synccatalog-enabled">`enabled`</a> (`boolean: false`) -
  If true, the chart will install all the resources necessary for the
  catalog sync process to run.

  - <a name="v-synccatalog-image" href="#v-synccatalog-image">`image`</a> (`string: global.imageK8S`) -
  The name of the Docker image (including any tag) for
  [consul-k8s](/docs/platform/k8s/index.html#quot-consul-k8s-quot-project)
  to run the sync program.

  - <a name="v-synccatalog-default" href="#v-synccatalog-default">`default`</a> (`boolean: true`) -
  If true, all valid services in K8S are synced by default. If false,
  the service must be [annotated](/docs/platform/k8s/service-sync.html#sync-enable-disable)
  properly to sync. In either case an annotation can override the default.

  - <a name="v-synccatalog-k8sprefix" href="#v-synccatalog-k8sprefix">`k8sPrefix`</a> (`string: ""`) -
  A prefix to prepend to all services registered in Kubernetes from Consul.
  This defaults to `""` where no prefix is prepended; Consul services are
  synced with the same name to Kubernetes.

  - <a name="v-synccatalog-toconsul" href="#v-synccatalog-toconsul">`toConsul`</a> (`boolean: true`) -
  If true, will sync Kubernetes services to Consul. This can be disabled to
  have a one-way sync.

  - <a name="v-synccatalog-tok8s" href="#v-synccatalog-tok8s">`toK8S`</a> (`boolean: true`) -
  If true, will sync Consul services to Kubernetes. This can be disabled to
  have a one-way sync.

* <a name="v-ui" href="#v-ui">`ui`</a> - Values that configure the Consul UI.

  - <a name="v-ui-enabled" href="#v-ui-enabled">`enabled`</a> (`boolean: global.enabled`) -
  If true, the UI will be enabled. This will only _enable_ the UI, it doesn't
  automatically register any service for external access. The UI will only
  be enabled on server agents. If `server.enabled` is false, then this setting
  has no effect. To expose the UI in some way, you must configure
  `ui.service`.

  - <a name="v-ui-service" href="#v-ui-service">`service`</a> -
  This configures the `Service` resource registered for the Consul UI.

      * <a name="v-ui-service-enabled" href="#v-ui-service-enabled">`enabled`</a> (`boolean: true`) -
      This will enable/disable registering a Kubernetes Service for the Consul UI.
      This value only takes effect if `ui.enabled` is true and taking effect.

      * <a name="v-ui-service-type" href="#v-ui-service-type">`type`</a> (`string: null`) -
      The service type to register. This defaults to `null` which doesn't set
      an explicit service type, which typically is defaulted to "ClusterIP"
      by Kubernetes. The available service types are documented on
      [the Kubernetes website](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types).

* <a name="v-connectinject" href="#v-connectinject">`connectInject`</a> - Values that
  configure running the [Connect injector](/docs/platform/k8s/connect.html).

  - <a name="v-connectinject-enabled" href="#v-connectinject-enabled">`enabled`</a> (`boolean: false`) -
  If true, the chart will install all the resources necessary for the
  Connect injector process to run. This will enable the injector but will
  require pods to opt-in with an annotation by default.

  - <a name="v-connectinject-image" href="#v-connectinject-image">`image`</a> (`string: global.imageK8S`) -
  The name of the Docker image (including any tag) for the
  [consul-k8s](https://github.com/hashicorp/consul-k8s) binary.

  - <a name="v-connectinject-default" href="#v-connectinject-default">`default`</a> (`boolean: false`) -
  If true, the injector will inject the Connect sidecar into all pods by
  default. Otherwise, pods must specify the
  [injection annotation](/docs/platform/k8s/connect.html#consul-hashicorp-com-connect-inject)
  to opt-in to Connect injection. If this is true, pods can use the same
  annotation to explicitly opt-out of injection.

  - <a name="v-connectinject-imageConsul" href="#v-connectinject-imageConsul">`imageConsul`</a> (`string: global.image`) -
  The name of the Docker image (including any tag) for Consul. This is used
  for proxy service registration, Envoy configuration, etc.

  - <a name="v-connectinject-imageEnvoy" href="#v-connectinject-imageEnvoy">`imageEnvoy`</a> (`string: ""`) -
  The name of the Docker image (including any tag) for the Envoy sidecar.
  `envoy` must be on the executable path within this image. This Envoy
  version must be compatible with the Consul version used by the injector.
  This defaults to letting the injector choose the Envoy image, which is
  usually `envoy/envoy-alpine`.

  - <a name="v-connectinject-namespaceselector" href="#v-connectinject-namespaceselector">`namespaceSelector`</a> (`string: ""`) -
  A [selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/)
  for restricting injection to only matching namespaces. By default
  all namespaces except the system namespace will have injection enabled.

  - <a name="v-connectinject-certs" href="#v-connectinject-certs">`certs`</a> -
  The certs section configures how the webhook TLS certs are configured.
  These are the TLS certs for the Kube apiserver communicating to the
  webhook. By default, the injector will generate and manage its own certs,
  but this requires the ability for the injector to update its own
  `MutatingWebhookConfiguration`. In a production environment, custom certs
  should probably be used. Configure the values below to enable this.

      * <a name="v-connectinject-certs-secretname" href="#v-connectinject-certs-secretname">`secretName`</a> (`string: null`) -
      secretName is the name of the Kubernetes secret that has the TLS certificate and
      private key to serve the injector webhook. If this is null, then the
      injector will default to its automatic management mode.

      * <a name="v-connectinject-cabundle" href="#v-connectinject-cabundle">`caBundle`</a> (`string: ""`) -
      The PEM-encoded CA public certificate bundle for the TLS certificate served by the
      injector. This must be specified as a string and can't come from a
      secret because it must be statically configured on the Kubernetes
      `MutatingAdmissionWebhook` resource. This only needs to be specified
      if `secretName` is not null.

      * <a name="v-connectinject-certs-certname" href="#v-connectinject-certs-certname">`certName`</a> (`string: "tls.crt"`) -
      The name of the certificate file within the `secretName` secret.

      * <a name="v-connectinject-certs-keynamkeyname" href="#v-connectinject-certs-keyname">`keyName`</a> (`string: "tls.key"`) -
      The name of the private key for the certificate file within the
      `secretName` secret.

## Customizing the Helm Chart

Given the wide variety of use cases, it won't be possible to support every
configuration option in the Helm chart's `values.yaml` file without making
it difficult to understand and use. For additional
flexibility in changing values that aren't listed above, consider using
third-party tools such as [kustomize](https://github.com/kubernetes-sigs/kustomize)
and [ship](https://github.com/replicatedhq/ship).
