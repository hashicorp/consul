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

~> **Security Warning:** By default, the chart will install an insecure
configuration of Consul. This provides a less complicated out-of-box experience
for new users, but is not appropriate for a production setup. Make sure that
your Kubernetes cluster is properly secured to prevent unwanted access to
Consul, or that you understand and enable the
[recommended Consul security features](/docs/internals/security.html).
Currently, some of these features are not supported in the Helm chart and
require additional manual configuration.

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

* <a name="v-global" href="#v-global">`global`</a> - These global values affect multiple components of the chart.

  * <a name="v-global-enabled" href="#v-global-enabled">`enabled`</a> (`boolean: true`) - The master enabled/disabled configuration. If this is true, most components will be installed by default. If this is false, no components will be installed by default and manually opt-in is required, such as by setting <a href="#v-">`server.enabled`</a> to true.

  * <a name="v-global-domain" href="#v-global-domain">`domain`</a> (`string: "consul"`) - The domain Consul uses for DNS queries. This is used to configure agents both for DNS listening but also to know what domain to join the cluster. This should be consistent throughout the chart, but can be overridden per-component as well.

  * <a name="v-global-image" href="#v-global-image">`image`</a> (`string: "consul:latest"`) - The name of the Docker image (including any tag) for the containers running Consul agents. **This should be pinned to a specific version when running in production.** Otherwise, other changes to the chart may inadvertently upgrade your Consul version.

  * <a name="v-global-imagek8s" href="#v-global-imagek8s">`imageK8S`</a> (`string: "hashicorp/consul-k8s:latest"`) - The name of the Docker image (including any tag) for the [consul-k8s](https://github.com/hashicorp/consul-k8s) binary. This is used by components such as catalog sync. **This should be pinned to a specific version when running in production.** Otherwise, other changes to the chart may inadvertently upgrade the version.

  * <a name="v-global-datacenter" href="#v-global-datacenter">`datacenter`</a> (`string: "dc1"`) - The name of the datacenter that the agent cluster should register as. This may not be changed once the cluster is bootstrapped and running, since Consul doesn't yet support an automatic way to change this value.

 * <a name="v-global-pod-security-policies" href="#v-pod-security-policies">`enablePodSecurityPolicies`</a> (`boolean: false`) -
  This flag controls whether [`PodSecurityPolicies`](https://kubernetes.io/docs/concepts/policy/pod-security-policy/) are created
  for the Consul components that this chart creates.

  * <a name="v-global-bootstrap-acls" href="#v-global-bootstrap-acls">`bootstrapACLs`</a> (`boolean: false`) - This flag controls
  whether the Helm chart automatically enables ACLs within the Consul cluster. This requires both Consul servers and clients to be run within
  Kubernetes. Requires Consul v1.5+ and consul-k8s v0.8.0+.

* <a name="v-server" href="#v-server">`server`</a> - Values that configure running a Consul server within Kubernetes.

  * <a name="v-server-enabled" href="#v-server-enabled">`enabled`</a> (`boolean: global.enabled`) - If true, the chart will install all the resources necessary for a Consul server cluster. If you're running Consul externally and want agents within Kubernetes to join that cluster, this should probably be false.

  * <a name="v-server-image" href="#v-server-image">`image`</a> (`string: global.image`) - The name of the Docker image (including any tag) for the containers running Consul server agents.

  * <a name="v-server-replicas" href="#v-server-replicas">`replicas`</a> (`integer: 3`) -The number of server agents to run. This determines the fault tolerance of the cluster. Please see the [deployment table](/docs/internals/consensus.html#deployment-table) for more information.

  * <a name="v-server-bootstrapexpect" href="#v-server-bootstrapexpect">`bootstrapExpect`</a> (`integer: 3`) - For new clusters, this is the number of servers to wait for before performing the initial leader election and bootstrap of the cluster. This must be less than or equal to `server.replicas`. This value is only used when bootstrapping new clusters, it has no effect during ongoing cluster maintenance.

  * <a name="v-server-storage" href="#v-server-storage">`storage`</a> (`string: 10Gi`) - This defines the disk size for configuring the servers' StatefulSet storage. For dynamically provisioned storage classes, this is the desired size. For manually defined persistent volumes, this should be set to the disk size of the attached volume.

  * <a name="v-server-storageclass" href="#v-server-storageclass">`storageClass`</a> (`string: null`) - The StorageClass to use for the servers' StatefulSet storage. It must be able to be dynamically provisioned if you want the storage to be automatically created. For example, to use [Local](https://kubernetes.io/docs/concepts/storage/storage-classes/#local) storage classes, the PersistentVolumeClaims would need to be manually created. A `null` value will use the Kubernetes cluster's default StorageClass. If a default StorageClass does not exist, you will need to create one.

  * <a name="v-server-connect" href="#v-server-connect">`connect`</a> (`boolean: true`) - This will enable/disable [Connect](/docs/connect/index.html). Setting this to true _will not_ automatically secure pod communication, this setting will only enable usage of the feature. Consul will automatically initialize a new CA and set of certificates. Additional Connect settings can be configured by setting the `server.extraConfig` value.

  * <a name="v-server-resources" href="#v-server-resources">`resources`</a> (`string: null`) - The resource requests (CPU, memory, etc.) for each of the server agents. This should be a multi-line string mapping directly to a Kubernetes [ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#resourcerequirements-v1-core) object. If this isn't specified, then the pods won't request any specific amount of resources. **Setting this is highly recommended.**

        ```yaml
        # Resources are defined as a formatted multi-line string:
        resources: |
          requests:
            memory: "10Gi"
          limits:
           memory: "10Gi"
        ```

  * <a name="v-server-updatepartition" href="#v-server-updatepartition">`updatePartition`</a> (`integer: 0`) - This value is used to carefully control a rolling update of Consul server agents. This value specifies the [partition](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#partitions) for performing a rolling update. Please read the linked Kubernetes documentation for more information.

  * <a name="v-server-disruptionbudget" href="#v-server-disruptionbudget">`disruptionBudget`</a> - This configures the [PodDisruptionBudget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) for the server cluster.

      - <a name="v-server-disruptionbudget-enabled" href="#v-server-disruptionbudget-enabled">`enabled`</a> (`boolean: true`) -
      This will enable/disable registering a PodDisruptionBudget for
      the server cluster. If this is enabled, it will only register the
      budget so long as the server cluster is enabled.

      - <a name="v-server-disruptionbudget-maxunavailable" href="#v-server-disruptionbudget-maxunavailable">`maxUnavailable`</a> (`integer: null`) -
      The maximum number of unavailable pods. By default, this will be automatically
      computed based on the `server.replicas` value to be `(n/2)-1`. If you need to set
      this to `0`, you will need to add a `--set 'server.disruptionBudget.maxUnavailable=0'`
      flag to the helm chart installation command because of a limitation in the Helm
      templating language.

  * <a name="v-server-extraconfig" href="#v-server-extraconfig">`extraConfig`</a> (`string: "{}"`) - A raw string of extra JSON [configuration](/docs/agent/options.html) for Consul servers. This will be saved as-is into a ConfigMap that is read by the Consul server agents. This can be used to add additional configuration that isn't directly exposed by the chart.

        ```yaml
        # ExtraConfig values are formatted as a multi-line string:
        extraConfig: |
          {
            "log_level": "DEBUG"
          }
        ```
        This can also be set using Helm's `--set` flag (consul-helm v0.7.0 and later), using the following syntax:

        ```shell
        --set 'server.extraConfig="{"log_level": "DEBUG"}"'
        ```

  * <a name="v-server-extravolumes" href="#v-server-extravolumes">`extraVolumes`</a> (`array: []`) - A list of extra volumes to mount for server agents. This is useful for bringing in extra data that can be referenced by other configurations at a well known path, such as TLS certificates or Gossip encryption keys. The value of this should be a list of objects. Each object supports the following keys:

      - <a name="v-server-extravolumes-type" href="#v-server-extravolumes-type">`type`</a> (`string: required`) -
      Type of the volume, must be one of "configMap" or "secret". Case sensitive.

      - <a name="v-server-extravolumes-name" href="#v-server-extravolumes-name">`name`</a> (`string: required`) -
      Name of the configMap or secret to be mounted. This also controls the path
      that it is mounted to. The volume will be mounted to `/config/userconfig/<name>`.

      - <a name="v-server-extravolumes-load" href="#v-server-extravolumes-load">`load`</a> (`boolean: false`) -
      If true, then the agent will be configured to automatically load HCL/JSON
      configuration files from this volume with `-config-dir`. This defaults
      to false.

        ```yaml
        extraVolumes:    
          -  type: "secret"
             name: "consul-certs"
             load: false        
        ```

  * <a name="v-server-affinity" href="#v-server-affinity">`affinity`</a> (`string`) - This value defines the [affinity](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity) for server pods. It defaults to allowing only a single pod on each node, which minimizes risk of the cluster becoming unusable if a node is lost. If you need to run more pods per node (for example, testing on Minikube), set this value to `null`.

        ```yaml
        # Recommended default server affinity:
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

  * <a name="v-server-priorityclassname" href="#v-server-priorityclassname">`priorityClassName`</a> (`string`) - This value references an existing Kubernetes [priorityClassName](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#pod-priority) that can be assigned to server pods.

  * <a name="v-server-annotations" href="#v-server-annotations">`annotations`</a> (`string`) - This value defines additional annotations for server pods. This should be a formatted as a multi-line string.

        ```yaml
        annotations: |
          "sample/annotation1": "foo"
          "sample/annotation2": "bar"
        ```

* <a name="v-client" href="#v-client">`client`</a> - Values that configure running a Consul client on Kubernetes nodes.

  * <a name="v-client-enabled" href="#v-client-enabled">`enabled`</a> (`boolean: global.enabled`) - If true, the chart will install all the resources necessary for a Consul client on every Kubernetes node. This _does not_ require `server.enabled`, since the agents can be configured to join an external cluster.

  * <a name="v-client-image" href="#v-client-image">`image`</a> (`string: global.image`) - The name of the Docker image (including any tag) for the containers running Consul client agents.

  * <a name="v-client-join" href="#v-client-join">`join`</a> (`array<string>: null`) - A list of valid [`-retry-join` values](/docs/agent/options.html#retry-join). If this is `null` (default), then the clients will attempt to automatically join the server cluster running within Kubernetes. This means that with `server.enabled` set to true, clients will automatically join that cluster. If `server.enabled` is not true, then a value must be specified so the clients can join a valid cluster.

  * <a name="v-client-grpc" href="#v-client-grpc">`grpc`</a> (`boolean: false`) - If true, agents will enable their GRPC listener on port 8502 and expose it to the host. This will use slightly more resources, but is required for [Connect](/docs/platform/k8s/connect.html).

  * <a name="v-client-resources" href="#v-client-resources">`resources`</a> (`string: null`) - The resource requests (CPU, memory, etc.) for each of the client agents. This should be a multi-line string mapping directly to a Kubernetes [ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#resourcerequirements-v1-core) object. If this isn't specified, then the pods won't request any specific amount of resources.

        ```yaml
        # Resources are defined as a formatted multi-line string:
        resources: |
          requests:
            memory: "10Gi"
          limits:
            memory: "10Gi"
        ```

  * <a name="v-client-extraconfig" href="#v-client-extraconfig">`extraConfig`</a> (`string: "{}"`) - A raw string of extra JSON [configuration](/docs/agent/options.html) for Consul clients. This will be saved as-is into a ConfigMap that is read by the Consul agents. This can be used to add additional configuration that isn't directly exposed by the chart.

        ```yaml
        # ExtraConfig values are formatted as a multi-line string:
        extraConfig: |
          {
            "log_level": "DEBUG"
          }
        ```
        This can also be set using Helm's `--set` flag (consul-helm v0.7.0 and later), using the following syntax:

        ```shell
        --set 'client.extraConfig="{"log_level": "DEBUG"}"'
        ```

  * <a name="v-client-extravolumes" href="#v-client-extravolumes">`extraVolumes`</a> (`array: []`) - A list of extra volumes to mount for client agents. This is useful for bringing in extra data that can be referenced by other configurations at a well known path, such as TLS certificates or Gossip encryption keys. The value of this should be a list of objects. Each object supports the following keys:

      - <a name="v-client-extravolumes-type" href="#v-client-extravolumes-type">`type`</a> (`string: required`) -
      Type of the volume, must be one of "configMap" or "secret". Case sensitive.

      - <a name="v-client-extravolumes-name" href="#v-client-extravolumes-name">`name`</a> (`string: required`) -
      Name of the configMap or secret to be mounted. This also controls the path
      that it is mounted to. The volume will be mounted to `/config/userconfig/<name>`.

      - <a name="v-client-extravolumes-load" href="#v-client-extravolumes-load">`load`</a> (`boolean: false`) -
      If true, then the agent will be configured to automatically load HCL/JSON
      configuration files from this volume with `-config-dir`. This defaults
      to false.

        ```yaml
        extraVolumes:    
          -  type: "secret"
             name: "consul-certs"
             load: false        
        ```

  * <a name="v-client-priorityclassname" href="#v-client-priorityclassname">`priorityClassName`</a> (`string`) - This value references an existing Kubernetes [priorityClassName](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#pod-priority) that can be assigned to client pods.

  * <a name="v-client-annotations" href="#v-client-annotations">`annotations`</a> (`string`) - This value defines additional annotations for client pods. This should be a formatted as a multi-line string.

        ```yaml
        annotations: |
          "sample/annotation1": "foo"
          "sample/annotation2": "bar"
        ```


* <a name="v-dns" href="#v-dns">`dns`</a> - Values that configure Consul DNS service.

  * <a name="v-dns-enabled" href="#v-dns-enabled">`enabled`</a> (`boolean: global.enabled`) - If true, a `consul-dns` service will be created that exposes port 53 for TCP and UDP to the running Consul agents (servers and clients). This can then be used to [configure kube-dns](/docs/platform/k8s/dns.html). The Helm chart _does not_ automatically configure kube-dns.

* <a name="v-synccatalog" href="#v-synccatalog">`syncCatalog`</a> - Values that configure the [service sync](/docs/platform/k8s/service-sync.html) process.

  * <a name="v-synccatalog-enabled" href="#v-synccatalog-enabled">`enabled`</a> (`boolean: false`) - If true, the chart will install all the resources necessary for the catalog sync process to run.

  * <a name="v-synccatalog-image" href="#v-synccatalog-image">`image`</a> (`string: global.imageK8S`) - The name of the Docker image (including any tag) for [consul-k8s](/docs/platform/k8s/index.html#quot-consul-k8s-quot-project)
to run the sync program.

  * <a name="v-synccatalog-default" href="#v-synccatalog-default">`default`</a> (`boolean: true`) - If true, all valid services in K8S are synced by default. If false, the service must be [annotated](/docs/platform/k8s/service-sync.html#sync-enable-disable) properly to sync. In either case an annotation can override the default.

  * <a name="v-synccatalog-toconsul" href="#v-synccatalog-toconsul">`toConsul`</a> (`boolean: true`) - If true, will sync Kubernetes services to Consul. This can be disabled to have a one-way sync.

  * <a name="v-synccatalog-tok8s" href="#v-synccatalog-tok8s">`toK8S`</a> (`boolean: true`) - If true, will sync Consul services to Kubernetes. This can be disabled to have a one-way sync.

  * <a name="v-synccatalog-k8sprefix" href="#v-synccatalog-k8sprefix">`k8sPrefix`</a> (`string: ""`) - A prefix to prepend to all services registered in Kubernetes from Consul. This defaults to `""` where no prefix is prepended; Consul services are synced with the same name to Kubernetes. (Consul -> Kubernetes sync only)

  * <a name="v-synccatalog-consulPrefix" href="#v-synccatalog-consulPrefix">`consulPrefix`</a> (`string: ""`) - A prefix to prepend to all services registered in Consul from Kubernetes. This defaults to `""` where no prefix is prepended. Service names within Kubernetes remain unchanged. (Kubernetes -> Consul sync only)

  * <a name="v-synccatalog-k8stag" href="#v-synccatalog-k8stag">`k8sTag`</a> (`string: null`) - An optional tag that is applied to all of the Kubernetes services that are synced into Consul. If nothing is set, this defaults to "k8s". (Kubernetes -> Consul sync only)

  * <a name="v-synccatalog-clusterip-sync" href="#v-synccatalog-clusterip-sync">`syncClusterIPServices`</a> (`boolean: true`) - If true, will sync Kubernetes ClusterIP services to Consul. This can be disabled to have the sync ignore ClusterIP-type services.

  * <a name="v-synccatalog-nodeport-sync" href="#v-synccatalog-nodeport-sync">`nodePortSyncType`</a> (`string: ExternalFirst`) - Configures the type of syncing that happens for NodePort services. The only valid options are: `ExternalOnly`, `InternalOnly`, and `ExternalFirst`. `ExternalOnly` will only use a node's ExternalIP address for the sync, otherwise the service will not be synced. `InternalOnly` uses the node's InternalIP address. `ExternalFirst` will preferentially use the node's ExternalIP address, but if it doesn't exist, it will use the node's InternalIP address instead.

  * <a name="v-synccatalog-acl-sync-token" href="#v-synccatalog-acl-sync-token">`aclSyncToken`</a> - references a Kubernetes [secret](https://kubernetes.io/docs/concepts/configuration/secret/#creating-your-own-secrets) that contains an existing Consul ACL token. This will provide the sync process the correct permissions. This is only needed if ACLs are enabled on the Consul cluster.

    - <a name="v-synccatalog-acl-sync-token-secret-name" href="#v-synccatalog-acl-sync-token-secret-name">secretName </a>`(string: null)` - The name of the Kubernetes secret. This defaults to null.

    - <a name="v-synccatalog-acl-sync-token-secret-key" href="#v-synccatalog-acl-sync-token-secret-key">secretKey </a>`(string: null)` - The key for the Kubernetes secret. This defaults to null.

* <a name="v-ui" href="#v-ui">`ui`</a> - Values that configure the Consul UI.

  * <a name="v-ui-enabled" href="#v-ui-enabled">`enabled`</a> (`boolean: global.enabled`) - If true, the UI will be enabled. This will only _enable_ the UI, it doesn't automatically register any service for external access. The UI will only be enabled on server agents. If `server.enabled` is false, then this setting has no effect. To expose the UI in some way, you must configure `ui.service`.

  * <a name="v-ui-service" href="#v-ui-service">`service`</a> - This configures the `Service` resource registered for the Consul UI.

      - <a name="v-ui-service-enabled" href="#v-ui-service-enabled">`enabled`</a> (`boolean: true`) -
      This will enable/disable registering a Kubernetes Service for the Consul UI.
      This value only takes effect if `ui.enabled` is true and taking effect.

      - <a name="v-ui-service-type" href="#v-ui-service-type">`type`</a> (`string: null`) -
      The service type to register. This defaults to `null` which doesn't set
      an explicit service type, which typically is defaulted to "ClusterIP"
      by Kubernetes. The available service types are documented on
      [the Kubernetes website](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types).

* <a name="v-connectinject" href="#v-connectinject">`connectInject`</a> - Values that configure running the [Connect injector](/docs/platform/k8s/connect.html).

  * <a name="v-connectinject-enabled" href="#v-connectinject-enabled">`enabled`</a> (`boolean: false`) - If true, the chart will install all the resources necessary for the Connect injector process to run. This will enable the injector but will require pods to opt-in with an annotation by default.

  * <a name="v-connectinject-image" href="#v-connectinject-image">`image`</a> (`string: global.imageK8S`) - The name of the Docker image (including any tag) for the [consul-k8s](https://github.com/hashicorp/consul-k8s) binary.

  * <a name="v-connectinject-default" href="#v-connectinject-default">`default`</a> (`boolean: false`) - If true, the injector will inject the Connect sidecar into all pods by default. Otherwise, pods must specify the. [injection annotation](/docs/platform/k8s/connect.html#consul-hashicorp-com-connect-inject) to opt-in to Connect injection. If this is true, pods can use the same annotation to explicitly opt-out of injection.

  * <a name="v-connectinject-imageConsul" href="#v-connectinject-imageConsul">`imageConsul`</a> (`string: global.image`) - The name of the Docker image (including any tag) for Consul. This is used for proxy service registration, Envoy configuration, etc.

  * <a name="v-connectinject-imageEnvoy" href="#v-connectinject-imageEnvoy">`imageEnvoy`</a> (`string: ""`) - The name of the Docker image (including any tag) for the Envoy sidecar. `envoy` must be on the executable path within this image. This Envoy version must be compatible with the Consul version used by the injector. This defaults to letting the injector choose the Envoy image, which is usually `envoy/envoy-alpine`.

  * <a name="v-connectinject-namespaceselector" href="#v-connectinject-namespaceselector">`namespaceSelector`</a> (`string: ""`) - A [selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) for restricting injection to only matching namespaces. By default all namespaces except the system namespace will have injection enabled.

  * <a name="v-connectinject-certs" href="#v-connectinject-certs">`certs`</a> - The certs section configures how the webhook TLS certs are configured. These are the TLS certs for the Kube apiserver communicating to the webhook. By default, the injector will generate and manage its own certs, but this requires the ability for the injector to update its own `MutatingWebhookConfiguration`. In a production environment, custom certs should probably be used. Configure the values below to enable this.

      - <a name="v-connectinject-certs-secretname" href="#v-connectinject-certs-secretname">`secretName`</a> (`string: null`) -
      secretName is the name of the Kubernetes secret that has the TLS certificate and
      private key to serve the injector webhook. If this is null, then the
      injector will default to its automatic management mode.

      - <a name="v-connectinject-cabundle" href="#v-connectinject-cabundle">`caBundle`</a> (`string: ""`) -
      The PEM-encoded CA public certificate bundle for the TLS certificate served by the
      injector. This must be specified as a string and can't come from a
      secret because it must be statically configured on the Kubernetes
      `MutatingAdmissionWebhook` resource. This only needs to be specified
      if `secretName` is not null.

      - <a name="v-connectinject-certs-certname" href="#v-connectinject-certs-certname">`certName`</a> (`string: "tls.crt"`) -
      The name of the certificate file within the `secretName` secret.

      - <a name="v-connectinject-certs-keynamkeyname" href="#v-connectinject-certs-keyname">`keyName`</a> (`string: "tls.key"`) -
      The name of the private key for the certificate file within the
      `secretName` secret.

  * <a name="v-connectinject-acl-bindingrule-selector" href="#v-connectinject-acl-bindingrule-selector">`namespaceSelector`</a> (`string: "serviceaccount.name!=default"`) -
  A [selector](/docs/acl/acl-auth-methods.html#binding-rules) for restricting automatic injection to only matching services based on
  their associated service account. By default, services using the `default` Kubernetes service account will not have a proxy injected.

  * <a name="v-connectinject-centralconfig" href="#v-connectinject-centralconfig">`centralConfig`</a> - Values that configure
  Consul's [central configuration](/docs/agent/config_entries.html) feature (requires Consul v1.5+ and consul-k8s v0.8.1+).

      - <a name="v-connectinject-centralconfig-enabled" href="#v-connectinject-centralconfig-enabled">`enabled`</a> (`boolean: false`) -
      Turns on the central configuration feature. Pods that have a Connect proxy injected will have their service
      automatically registered in this central configuration.

      - <a name="v-connectinject-centralconfig-defaultprotocol" href="#v-connectinject-centralconfig-defaultprotocol">`defaultProtocol`</a> (`string: null`) -
      If defined, this value will be used as the default protocol type for all services registered with the central configuration.
      This can be overridden by using the
      [protocol annotation](/docs/platform/k8s/connect.html#consul-hashicorp-com-connect-service-protocol)
      directly on any pod spec.

      - <a name="v-connectinject-centralconfig-proxydefaults" href="#v-connectinject-centralconfig-proxydefaults">`proxyDefaults`</a> (`string: "{}"`) -
      This value is a raw json string that will be applied to all Connect proxy sidecar pods. It can include any valid configuration
      for the configured proxy.

        ```yaml
        # proxyDefaults values are formatted as a multi-line string:
        proxyDefaults: |
          {
            "envoy_dogstatsd_url": "udp://127.0.0.1:9125"
          }
        ```

## Using the Helm Chart to deploy Consul Enterprise

You can also use this Helm chart to deploy Consul Enterprise by following a few extra steps.

Find the license file that you received in your welcome email. It should have the extension `.hclic`. You will use the contents of this file to create a Kubernetes secret before installing the Helm chart.

-> **Note:** If you cannot find your `.hclic` file, please contact your sales team or Technical Account Manager.

You can use the following commands to create the secret:

```bash
secret=$(cat 1931d1f4-bdfd-6881-f3f5-19349374841f.hclic)
kubectl create secret generic consul-ent-license --from-literal="key=${secret}"
```

In your `values.yaml`, change the value of `global.image` to one of the enterprise [release tags](https://hub.docker.com/r/hashicorp/consul-enterprise/tags).

```yaml
global:
  image: "hashicorp/consul-enterprise:1.4.3-ent"
```

Add the name of the secret you just created to `server.enterpriseLicense`.

```yaml
server:
  enterpriseLicense:
    secretName: "consul-ent-license"
    secretKey: "key"
```

Add the `--wait` option to your `helm install` command. This will force Helm to wait for all the pods
to become ready before it applies the license to your Consul cluster.

```bash
$ helm install --wait .
```

Once the cluster is up, you can verify the nodes are running Consul Enterprise.

```bash
$ kubectl port-forward service/consul-server 8500 &
$ consul license get
License is valid
License ID: 1931d1f4-bdfd-6881-f3f5-19349374841f
Customer ID: b2025a4a-8fdd-f268-95ce-1704723b9996
Expires At: 2020-03-09 03:59:59.999 +0000 UTC
Datacenter: *
Package: premium
Licensed Features:
        Automated Backups
        Automated Upgrades
        Enhanced Read Scalability
        Network Segments
        Redundancy Zone
        Advanced Network Federation
$ consul members
Node                                       Address           Status  Type    Build      Protocol  DC   Segment
consul-server-0                            10.60.0.187:8301  alive   server  1.4.3+ent  2         dc1  <all>
consul-server-1                            10.60.1.229:8301  alive   server  1.4.3+ent  2         dc1  <all>
consul-server-2                            10.60.2.197:8301  alive   server  1.4.3+ent  2         dc1  <all>
```

## Helm Chart Examples

The below values.yaml can be used to set up a single server Consul cluster with a LoadBalancer to allow external access to the UI and API.

```
global:
  enabled: true
  image: "consul:1.4.2"
  domain: consul
  datacenter: dc1

server:
  enabled: true
  replicas: 1
  bootstrapExpect: 1
  storage: 10Gi

client:
  enabled: true

dns:
  enabled: true

ui:
  enabled: true
  service:
    enabled: true
    type: LoadBalancer
```

The below values.yaml can be used to set up a three server Consul Enterprise cluster with 100GB of storage and automatic Connect injection for annotated pods in the "my-app" namespace.

Note, this would require a secret that contains the enterprise license key.

```
global:
  enabled: true
  domain: consul
  image: "hashicorp/consul-enterprise:1.4.2-ent"
  datacenter: dc1

server:
  enabled: true
  replicas: 3
  bootstrapExpect: 3
  enterpriseLicense:
    secretName: "consul-license"
    secretKey: "key"
  storage: 100Gi
  connect: true
  affinity: |
    podAntiAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        - labelSelector:
          matchLabels:
            app: {{ template "consul.name" . }}
            release: "{{ .Release.Name }}"
            component: server
        topologyKey: kubernetes.io/hostname

client:
  enabled: true
  grpc: true

dns:
  enabled: true

ui:
  enabled: true
  service:
    enabled: true
    type: NodePort

connectInject:
  enabled: true
  default: false
  namespaceSelector: "my-app"

```

## Customizing the Helm Chart

Consul within Kubernetes is highly configurable and the Helm chart contains dozens of the most commonly used configuration options. If you need to extend the Helm chart with additional options, we recommend using a third-party tool, such as [kustomize](https://github.com/kubernetes-sigs/kustomize) or [ship](https://github.com/replicatedhq/ship).
