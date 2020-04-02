---
layout: "docs"
page_title: "Helm Chart Reference - Kubernetes"
sidebar_current: "docs-platform-k8s-helm"
description: |-
  Reference for the Consul Helm chart.
---

# Helm Chart Reference

## Configuration (Values)

The chart is highly customizable using
[Helm configuration values](https://helm.sh/docs/intro/using_helm/#customizing-the-chart-before-installing).
Each value has a sane default tuned for an optimal getting started experience
with Consul. Before going into production, please review the parameters below
and consider if they're appropriate for your deployment.

* <a name="v-global" href="#v-global">`global`</a>- Holds values that affect
  multiple components of the chart.

  * <a name="v-global-enabled" href="#v-global-enabled">`enabled`</a> (`boolean: true`) - The master enabled/disabled setting. If true, servers, clients, Consul DNS and the Consul UI will be enabled. Each component can override this default via its component-specific "enabled" config. If false, no components will be installed by default and per-component opt-in is required, such as by setting <a href="#v-server-enabled">`server.enabled`</a> to true.

  * <a name="v-global-name" href="#v-global-name">`name`</a> (`string: null`) - Set the prefix used for all resources in the Helm chart. If not set, the prefix will be `<helm release name>-consul`.

  * <a name="v-global-domain" href="#v-global-domain">`domain`</a> (`string: "consul"`) - The domain Consul will answer DNS queries for (see [-domain](/docs/agent/options.html#_domain)) and the domain services synced from Consul into Kubernetes will have, e.g. `service-name.service.consul`.

  * <a name="v-global-image" href="#v-global-image">`image`</a> (`string: "consul:<latest version>"`) - The name (and tag) of the Consul Docker image for clients and servers. This can be overridden per component. This should be pinned to a specific version tag, otherwise you may inadvertently upgrade your Consul version.

        Examples:

        ```yaml
        # Consul 1.5.0
        image: "consul:1.5.0"
        # Consul Enterprise 1.5.0
        image: "hashicorp/consul-enterprise:1.5.0-ent"
        ```

  * <a name="v-global-imagek8s" href="#v-global-imagek8s">`imageK8S`</a> (`string: "hashicorp/consul-k8s:<latest version>"`) - The name (and tag) of the [consul-k8s](https://github.com/hashicorp/consul-k8s) Docker image that is used for functionality such the catalog sync. This can be overridden per component.

        Note: support for the catalog sync's liveness and readiness probes was added to consul-k8s 0.6.0. If using an older consul-k8s version, you may need to remove these checks to make sync work. If using mesh gateways and bootstrapACLs then must be >= 0.9.0.

  * <a name="v-global-datacenter" href="#v-global-datacenter">`datacenter`</a> (`string: "dc1"`) - The name of the datacenter that the agents should register as. This can't be changed once the Consul cluster is up and running since Consul doesn't support an automatic way to change this value currently: [https://github.com/hashicorp/consul/issues/1858](https://github.com/hashicorp/consul/issues/1858).

  * <a name="v-global-enablepodsecuritypolicies" href="#v-global-enablepodsecuritypolicies">`enablePodSecurityPolicies`</a> (`boolean: false`) -
        Controls whether pod security policies are created for the Consul components created by this chart. See [https://kubernetes.io/docs/concepts/policy/pod-security-policy/](https://kubernetes.io/docs/concepts/policy/pod-security-policy/).

  * <a name="v-global-gossipencryption" href="#v-global-gossipencryption">`gossipEncryption`</a> -
        Configures which Kubernetes secret to retrieve Consul's gossip encryption key from (see [-encrypt](/docs/agent/options.html#_encrypt)). If secretName or secretKey are not set, gossip encryption will not be enabled. The secret must be in the same namespace that Consul is installed into.

        The secret can be created by running:

        ```bash
        $ kubectl create secret generic consul-gossip-encryption-key --from-literal=key=$(consul keygen)
        # To reference, use:
        #   gossipEncryption:
        #     secretName: consul-gossip-encryption-key
        #     secretKey: key
        ```

        * <a name="v-global-gossipencryption-secretname" href="#v-global-gossipencryption-secretname">`secretName`</a> (`string: ""`) - The name of the Kubernetes secret that holds the gossip encryption key. The secret must be in the same namespace that Consul is installed into.

        * <a name="v-global-gossipencryption-secretkey" href="#v-global-gossipencryption-secretkey">`secretKey`</a> (`string: ""`) - The key within the Kubernetes secret that holds the gossip encryption key.

  * <a name="v-global-enableconsulnamespaces" href="#v-global-enableconsulnamespaces">`enableConsulNamespaces`</a> (`boolean: false`) - [Enterprise Only] `enableConsulNamespaces` indicates that you are running Consul Enterprise v1.7+ with a valid Consul Enterprise license and would like to make use of configuration beyond registering everything into the `default` Consul namespace. Requires consul-k8s v0.12+. Additional configuration options are found in the `consulNamespaces` section of both the catalog sync and connect injector.
  
  * <a name="v-global-bootstrapacls" href="#v-global-bootstrapacls">`bootstrapACLs`</a> (`boolean: false`) - **[DEPRECATED]** Use `global.acls.manageSystemACLs` instead.

  * <a name="v-global-acls" href="#v-global-acls">`acls`</a> - Configure ACLs.

      * <a name="v-global-acls-managesystemacls" href="#v-global-acls-managesystemacls">`manageSystemACLs`</a> (`boolean: false`) - If true, the Helm chart will automatically manage ACL tokens and policies for all Consul components. This requires servers to be running inside Kubernetes. Additionally requires Consul >= 1.4 and consul-k8s >= 0.10.1.

  * <a name="v-global-tls" href="#v-global-tls">`tls`</a> - Enables TLS [encryption](https://learn.hashicorp.com/consul/security-networking/agent-encryption) across the cluster to verify authenticity of the Consul servers and clients. Requires Consul v1.4.1+ and consul-k8s v0.16.2+

      * <a name="v-global-tls-enabled" href="#v-global-enabled">`enabled`</a> (`boolean: false`) - If true, the Helm chart will enable TLS for Consul servers and clients and all consul-k8s components, as well as generate certificate authority (optional) and server and client certificates.

      * <a name="v-global-tls-serveradditionaldnsssans" href="#v-global-serveradditionaldnsssans">`serverAdditionalDNSSANs`</a> (`array<string>: []`) - A list of additional DNS names to set as Subject Alternative Names (SANs) in the server certificate. This is useful when you need to access the Consul server(s) externally, for example, if you're using the UI.

      * <a name="v-global-tls-serveradditionalipsans" href="#v-global-serveradditionalipsans">`serverAdditionalIPSANs`</a> (`array<string>: []`) - A list of additional IP addresses to set as Subject Alternative Names (SANs) in the server certificate. This is useful when you need to access the Consul server(s) externally, for example, if you're using the UI.

      * <a name="v-global-tls-verify" href="#v-global-verify">`verify`</a> (`boolean: true`) - If true, `verify_outgoing`, `verify_server_hostname`, and `verify_incoming_rpc` will be set to `true` for Consul servers and clients.
      Set this to false to incrementally roll out TLS on an existing Consul cluster.
      Please see [Configuring TLS on an Existing Cluster](https://www.consul.io/docs/platform/k8s/tls-on-existing-cluster.html) for more details.

      * <a name="v-global-tls-httpsonly" href="#v-global-httpsonly">`httpsOnly`</a> (`boolean: true`) - If true, the Helm chart will configure Consul to disable the HTTP port on both clients and servers and to only accept HTTPS connections.

      * <a name="v-global-tls-cacert" href="#v-global-cacert">`caCert`</a> - A Kubernetes secret containing the certificate of the CA to use for TLS communication within the Consul cluster.
      If you have generated the CA yourself with the consul CLI, you could use the following command to create the secret in Kubernetes:

            ```bash
            kubectl create secret generic consul-ca-cert \
                    --from-file='tls.crt=./consul-agent-ca.pem'
            ```

            *  <a name="v-global-tls-cacert-secretname" href="#v-global-cacert-secretname">`secretName`</a> (`string: null`) - The name of the Kubernetes secret.

            *  <a name="v-global-tls-cacert-secretkey" href="#v-global-cacert-secretkey">`secretKey`</a> (`string: null`) - The key of the Kubernetes secret.

      * <a name="v-global-tls-cakey" href="#v-global-cakey">`caKey`</a> - A Kubernetes secret containing the private key of the CA to use for TLS communication within the Consul cluster.
      If you have generated the CA yourself with the consul CLI, you could use the following command to create the secret in Kubernetes:

            ```bash
            kubectl create secret generic consul-ca-key \
                    --from-file='tls.key=./consul-agent-ca-key.pem'
            ```

            *  <a name="v-global-tls-cakey-secretname" href="#v-global-cakey-secretname">`secretName`</a> (`string: null`) - The name of the Kubernetes secret.

            *  <a name="v-global-tls-cakey-secretkey" href="#v-global-cakey-secretkey">`secretKey`</a> (`string: null`) - The key of the Kubernetes secret.

* <a name="v-server" href="#v-server">`server`</a> - Values that configure running a Consul server within Kubernetes.

  * <a name="v-server-enabled" href="#v-server-enabled">`enabled`</a> (`boolean: global.enabled`) - If true, the chart will install all the resources necessary for a Consul server cluster. If you're running Consul externally and want agents within Kubernetes to join that cluster, this should probably be false.

  * <a name="v-server-image" href="#v-server-image">`image`</a> (`string: global.image`) - The name of the Docker image (including any tag) for the containers running Consul server agents.

  * <a name="v-server-replicas" href="#v-server-replicas">`replicas`</a> (`integer: 3`) -The number of server agents to run. This determines the fault tolerance of the cluster. Please see the [deployment table](/docs/internals/consensus.html#deployment-table) for more information.

  * <a name="v-server-bootstrapexpect" href="#v-server-bootstrapexpect">`bootstrapExpect`</a> (`integer: 3`) - For new clusters, this is the number of servers to wait for before performing the initial leader election and bootstrap of the cluster. This must be less than or equal to `server.replicas`. This value is only used when bootstrapping new clusters, it has no effect during ongoing cluster maintenance.

  * <a name="v-server-enterpriselicense" href="#v-server-enterpriselicense">`enterpriseLicense`</a> [Enterprise Only] - This value refers to a Kubernetes secret that you have created that contains your enterprise license. It is required if you are using an enterprise binary. Defining it here applies it to your cluster once a leader has been elected. If you are not using an enterprise image or if you plan to introduce the license key via another route, then set these fields to null.

      * <a name="v-global-enterpriselicense-secretname" href="#v-global-enterpriselicense-secretname">`secretName`</a> (`string: null`) - The name of the Kubernetes secret that holds the enterprise license. The secret must be in the same namespace that Consul is installed into.

      * <a name="v-global-enterpriselicense-secretkey" href="#v-global-enterpriselicense-secretkey">`secretKey`</a> (`string: null`) - The key within the Kubernetes secret that holds the enterprise license.

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
        that it is mounted to. The volume will be mounted to `/consul/userconfig/<name>`.

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

  * <a name="v-server-tolerations" href="#v-server-tolerations">`tolerations`</a> (`string: ""`) - Toleration settings for server pods. This should be a multi-line string matching the [Tolerations](https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/) array in a Pod spec.

  * <a name="v-server-nodeselector" href="#v-server-nodeselector">`nodeSelector`</a> (`string: null`) - This value defines [`nodeSelector`](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector) labels for server pod assignment, formatted as a multi-line string.

        ```yaml
        nodeSelector: |
          beta.kubernetes.io/arch: amd64
        ```

  * <a name="v-server-priorityclassname" href="#v-server-priorityclassname">`priorityClassName`</a> (`string`) - This value references an existing Kubernetes [priorityClassName](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#pod-priority) that can be assigned to server pods.

  * <a name="v-server-annotations" href="#v-server-annotations">`annotations`</a> (`string`) - This value defines additional annotations for server pods. This should be a formatted as a multi-line string.

        ```yaml
        annotations: |
          "sample/annotation1": "foo"
          "sample/annotation2": "bar"
        ```
  * <a name="v-server-service" href="#v-server-service">`service`</a> - Server service properties

      * <a name="v-server-service" href="#v-server-service-annotations">`annotations`</a> Annotations to apply to the server service.

            ```yaml
            annotations: |
              "annotation-key": "annotation-value"
            ```

* <a name="v-client" href="#v-client">`client`</a> - Values that configure running a Consul client on Kubernetes nodes.

  * <a name="v-client-enabled" href="#v-client-enabled">`enabled`</a> (`boolean: global.enabled`) - If true, the chart will install all the resources necessary for a Consul client on every Kubernetes node. This _does not_ require `server.enabled`, since the agents can be configured to join an external cluster.

  * <a name="v-client-image" href="#v-client-image">`image`</a> (`string: global.image`) - The name of the Docker image (including any tag) for the containers running Consul client agents.

  * <a name="v-client-join" href="#v-client-join">`join`</a> (`array<string>: null`) - A list of valid [`-retry-join` values](/docs/agent/options.html#retry-join). If this is `null` (default), then the clients will attempt to automatically join the server cluster running within Kubernetes. This means that with `server.enabled` set to true, clients will automatically join that cluster. If `server.enabled` is not true, then a value must be specified so the clients can join a valid cluster.

  * <a name="v-client-datadirectorypath" href="#v-client-datadirectorypath">`dataDirectoryPath`</a> (`string: null`) - An absolute path to a directory on the host machine to use as the Consul client data directory. If set to the empty string or null, the Consul agent will store its data in the Pod's local filesystem (which will be lost if the Pod is deleted). Security Warning: If setting this, Pod Security Policies *must* be enabled on your cluster and in this Helm chart (via the global.enablePodSecurityPolicies setting) to prevent other Pods from mounting the same host path and gaining access to all of Consul's data. Consul's data is not encrypted at rest.

  * <a name="v-client-grpc" href="#v-client-grpc">`grpc`</a> (`boolean: true`) - If true, agents will enable their GRPC listener on port 8502 and expose it to the host. This will use slightly more resources, but is required for [Connect](/docs/platform/k8s/connect.html).

  * <a name="v-client-exposegossipports" href="#v-client-exposegossipports">`exposeGossipPorts`</a> (`boolean: false`) - If true, the Helm chart will expose the clients' gossip ports as hostPorts. This is only necessary if pod IPs in the k8s cluster are not directly routable and the Consul servers are outside of the k8s cluster. This also changes the clients' advertised IP to the `hostIP` rather than `podIP`.

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
      that it is mounted to. The volume will be mounted to `/consul/userconfig/<name>`.

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

  * <a name="v-client-tolerations" href="#v-client-tolerations">`tolerations`</a> (`string: ""`) - Toleration Settings for client pods. This should be a multi-line string matching the Toleration array in a Pod spec. The example below will allow client pods to run on every node regardless of taints.

        ```yaml
        tolerations: |
          - operator: "Exists"
        ```

  * <a name="v-client-nodeselector" href="#v-client-nodeselector">`nodeSelector`</a> (`string: null`) - Labels for client pod assignment, formatted as a multi-line string. Please see [Kubernetes docs](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector) for more details.

        ```yaml
        nodeSelector: |
          beta.kubernetes.io/arch: amd64
        ```

  * <a name="v-client-priorityclassname" href="#v-client-priorityclassname">`priorityClassName`</a> (`string: ""`) - This value references an existing Kubernetes [priorityClassName](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#pod-priority) that can be assigned to client pods.

  * <a name="v-client-annotations" href="#v-client-annotations">`annotations`</a> (`string: null`) - This value defines additional annotations for client pods. This should be a formatted as a multi-line string.

        ```yaml
        annotations: |
          "sample/annotation1": "foo"
          "sample/annotation2": "bar"
        ```

  * <a name="v-client-dnspolicy" href="#v-client-dnspolicy">`dnsPolicy`</a> (`string: null`) - This value defines the [Pod DNS policy](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy) for client pods to use.

  * <a name="v-client-updatestrategy" href="#v-client-updatestrategy">`updateStrategy`</a> (`string: null`) - The [update strategy](https://kubernetes.io/docs/tasks/manage-daemon/update-daemon-set/#daemonset-update-strategy) for the client `DaemonSet`.

        ```yaml
        updateStrategy: |
          rollingUpdate:
            maxUnavailable: 5
          type: RollingUpdate
        ```

  * <a name="v-client-snapshotagent" href="#v-client-snapshotagent">`snapshotAgent`</a> [Enterprise Only] - Values for setting up and running [snapshot agents](https://www.consul.io/docs/commands/snapshot/agent.html) within the Consul clusters. They are required to be co-located with Consul clients, so will inherit the clients' nodeSelector, tolerations and affinity.

      * <a name="v-client-snapshotagent-enabled" href="#v-client-snapshotagent-enabled">`enabled`</a> (`boolean: false`) - If true, the chart will install resources necessary to run the snapshot agent.

      * <a name="v-client-snapshotagent-replicas" href="#v-client-snapshotagent-replicas">`replicas`</a> (`integer: 2`) - The number of snapshot agents to run.

      * <a name="v-client-snapshotagent-configsecret" href="#v-client-snapshotagent-configsecret">`configSecret`</a> - A Kubernetes secret that should be manually created to contain the entire config to be used on the snapshot agent. This is the preferred method of configuration since there are usually storage credentials present. Please see [Snapshot agent config](https://www.consul.io/docs/commands/snapshot/agent.html#config-file-options-) for details.

          * <a name="v-client-snapshotagent-configsecret-secretname" href="#v-client-snapshotagent-configsecret-secretname">secretName </a>`(string: null)` - The name of the Kubernetes secret.

          * <a name="v-client-snapshotagent-configsecret-secretkey" href="#v-client-snapshotagent-configsecret-secretkey">secretKey </a>`(string: null)` - The key for the Kubernetes secret.

* <a name="v-dns" href="#v-dns">`dns`</a> - Values that configure Consul DNS service.

  * <a name="v-dns-enabled" href="#v-dns-enabled">`enabled`</a> (`boolean: global.enabled`) - If true, a `consul-dns` service will be created that exposes port 53 for TCP and UDP to the running Consul agents (servers and clients). This can then be used to [configure kube-dns](/docs/platform/k8s/dns.html). The Helm chart _does not_ automatically configure kube-dns.

  * <a name="v-dns-clusterip" href="#v-dns-clusterip">`clusterIP`</a> (`string: null`) - If defined, this value configures the cluster IP of the DNS service.

  * <a name="v-dns-annotations" href="#v-dns-annotations">`annotations`</a> (`string: null`) - Extra annotations to attach to the DNS service. This should be a multi-line string of annotations to apply to the DNS service.

* <a name="v-synccatalog" href="#v-synccatalog">`syncCatalog`</a> - Values that configure the [service sync](/docs/platform/k8s/service-sync.html) process.

  * <a name="v-synccatalog-enabled" href="#v-synccatalog-enabled">`enabled`</a> (`boolean: false`) - If true, the chart will install all the resources necessary for the catalog sync process to run.

  * <a name="v-synccatalog-image" href="#v-synccatalog-image">`image`</a> (`string: global.imageK8S`) - The name of the Docker image (including any tag) for [consul-k8s](/docs/platform/k8s/index.html#quot-consul-k8s-quot-project)
to run the sync program.

  * <a name="v-synccatalog-default" href="#v-synccatalog-default">`default`</a> (`boolean: true`) - If true, all valid services in K8S are synced by default. If false, the service must be [annotated](/docs/platform/k8s/service-sync.html#sync-enable-disable) properly to sync. In either case an annotation can override the default.

  * <a name="v-synccatalog-toconsul" href="#v-synccatalog-toconsul">`toConsul`</a> (`boolean: true`) - If true, will sync Kubernetes services to Consul. This can be disabled to have a one-way sync.

  * <a name="v-synccatalog-tok8s" href="#v-synccatalog-tok8s">`toK8S`</a> (`boolean: true`) - If true, will sync Consul services to Kubernetes. This can be disabled to have a one-way sync.

  * <a name="v-synccatalog-k8sprefix" href="#v-synccatalog-k8sprefix">`k8sPrefix`</a> (`string: ""`) - A prefix to prepend to all services registered in Kubernetes from Consul. This defaults to `""` where no prefix is prepended; Consul services are synced with the same name to Kubernetes. (Consul -> Kubernetes sync only)
  
  * <a name="v-synccatalog-k8sallownamespaces" href="#v-synccatalog-k8sallownamespaces">`k8sAllowNamespaces`</a> (`[]string: ["*"]`) - list of k8s namespaces to sync the k8s services from. If a k8s namespace is not included in this list or is listed in `k8sDenyNamespaces`, services in that k8s namespace will not be synced even if they are explicitly annotated. Use `["*"]` to automatically allow all k8s namespaces.
     
         For example, `["namespace1", "namespace2"]` will only allow services in the k8s namespaces `namespace1` and `namespace2` to be synced and registered with Consul. All other k8s namespaces will be ignored.
         
         Note: `k8sDenyNamespaces` takes precedence over values defined here. Requires consul-k8s v0.12+
         
  * <a name="v-synccatalog-k8sdenynamespaces" href="#v-synccatalog-k8sdenynamespaces">`k8sDenyNamespaces`</a> (`[]string: ["kube-system", "kube-public"]` - list of k8s namespaces that should not have their services synced. This list takes precedence over `k8sAllowNamespaces`. `*` is not supported because then nothing would be allowed to sync. Requires consul-k8s v0.12+.
  
        For example, if `k8sAllowNamespaces` is `["*"]` and `k8sDenyNamespaces` is `["namespace1", "namespace2"]`, then all k8s namespaces besides `namespace1` and `namespace2` will be synced. 

  * <a name="v-synccatalog-k8ssourcenamespace" href="#v-synccatalog-k8ssourcenamespace">`k8sSourceNamespace`</a> (`string: ""`) - **[DEPRECATED] Use `k8sAllowNamespaces` and `k8sDenyNamespaces` instead.** `k8sSourceNamespace` is the Kubernetes namespace to watch for service changes and sync to Consul. If this is not set then it will default to all namespaces.
  
  * <a name="v-synccatalog-consulnamespaces" href="#v-synccatalog-consulnamespaces">`consulNamespaces`</a> -  [Enterprise Only] These settings manage the catalog sync's interaction with Consul namespaces (requires consul-ent v1.7+ and consul-k8s v0.12+). Also, `global.enableConsulNamespaces` must be true.
  
      * <a name="v-synccatalog-consulnamespaces-consuldestinationnamespace" href="#v-synccatalog-consulnamespaces-consuldestinationnamespace">`consulDestinationNamespace`</a> (`string: "default"`) - Name of the Consul namespace to register all k8s services into. If the Consul namespace does not already exist, it will be created. This will be ignored if `mirroringK8S` is true.
      
      * <a name="v-synccatalog-consulnamespaces-mirroringk8s" href="#v-synccatalog-consulnamespaces-mirroringk8s">`mirroringK8S`</a> (`bool: false`) - causes k8s services to be registered into a Consul namespace of the same name as their k8s namespace, optionally prefixed if `mirroringK8SPrefix` is set below. If the Consul namespace does not already exist, it will be created. Turning this on overrides the `consulDestinationNamespace` setting. `addK8SNamespaceSuffix` may no longer be needed if enabling this option.
      
      * <a name="v-synccatalog-consulnamespaces-mirroringk8sprefix" href="#v-synccatalog-consulnamespaces-mirroringk8sprefix">`mirroringK8SPrefix`</a> (`string: ""`) - If `mirroringK8S` is set to true, `mirroringK8SPrefix` allows each Consul namespace to be given a prefix. For example, if `mirroringK8SPrefix` is set to `"k8s-"`, a service in the k8s `staging` namespace will be registered into the `k8s-staging` Consul namespace.
  
  * <a name="v-synccatalog-addk8snamespacesuffix" href="#v-synccatalog-addk8snamespacesuffix">`addK8SNamespaceSuffix`</a> (`boolean: true`) - If true, sync catalog will append Kubernetes namespace suffix to each service name synced to Consul, separated by a dash.
  For example, for a service `foo` in the `default` namespace, the sync process will create a Consul service named `foo-default`.
  Set this flag to true to avoid registering services with the same name but in different namespaces as instances for the same Consul service.
  Namespace suffix is not added if `annotationServiceName` is provided.

  * <a name="v-synccatalog-consulPrefix" href="#v-synccatalog-consulPrefix">`consulPrefix`</a> (`string: ""`) - A prefix to prepend to all services registered in Consul from Kubernetes. This defaults to `""` where no prefix is prepended. Service names within Kubernetes remain unchanged. (Kubernetes -> Consul sync only)

  * <a name="v-synccatalog-k8stag" href="#v-synccatalog-k8stag">`k8sTag`</a> (`string: null`) - An optional tag that is applied to all of the Kubernetes services that are synced into Consul. If nothing is set, this defaults to "k8s". (Kubernetes -> Consul sync only)

  * <a name="v-synccatalog-syncclusteripservices" href="#v-synccatalog-syncclusteripservices">`syncClusterIPServices`</a> (`boolean: true`) - If true, will sync Kubernetes ClusterIP services to Consul. This can be disabled to have the sync ignore ClusterIP-type services.

  * <a name="v-synccatalog-nodeportsynctype" href="#v-synccatalog-nodeportsynctype">`nodePortSyncType`</a> (`string: ExternalFirst`) - Configures the type of syncing that happens for NodePort services. The only valid options are: `ExternalOnly`, `InternalOnly`, and `ExternalFirst`. `ExternalOnly` will only use a node's ExternalIP address for the sync, otherwise the service will not be synced. `InternalOnly` uses the node's InternalIP address. `ExternalFirst` will preferentially use the node's ExternalIP address, but if it doesn't exist, it will use the node's InternalIP address instead.

  * <a name="v-synccatalog-acl-sync-token" href="#v-synccatalog-acl-sync-token">`aclSyncToken`</a> - references a Kubernetes [secret](https://kubernetes.io/docs/concepts/configuration/secret/#creating-your-own-secrets) that contains an existing Consul ACL token. This will provide the sync process the correct permissions. This is only needed if ACLs are enabled on the Consul cluster.

      - <a name="v-synccatalog-acl-sync-token-secret-name" href="#v-synccatalog-acl-sync-token-secret-name">secretName </a>`(string: null)` - The name of the Kubernetes secret. This defaults to null.

      - <a name="v-synccatalog-acl-sync-token-secret-key" href="#v-synccatalog-acl-sync-token-secret-key">secretKey </a>`(string: null)` - The key for the Kubernetes secret. This defaults to null.

  * <a name="v-synccatalog-nodeselector" href="#v-synccatalog-nodeselector">`nodeSelector`</a> (`string: null`) - This value defines [`nodeSelector`](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector) labels for `syncCatalog` pod assignment, formatted as a multi-line string.

        ```yaml
        nodeSelector: |
          beta.kubernetes.io/arch: amd64
        ```

  * <a name="v-synccatalog-loglevel" href="#v-synccatalog-loglevel">`logLevel`</a> (`string: info`) - Log verbosity level. One of "trace", "debug", "info", "warn", or "error".

  * <a name="v-synccatalog-consulwriteinterval" href="#v-synccatalog-consulwriteinterval">`consulWriteInterval`</a> (`string: null`) - Override the default interval to perform syncing operations creating Consul services.

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

      - <a name="v-ui-service-annotations" href="#v-ui-service-annotations">`annotations`</a> (`string: null`) - Annotations to apply to the UI service.

            ```yaml
            annotations: |
              "annotation-key": "annotation-value"
            ```

      - <a name="v-ui-service-additionalspec" href="#v-ui-service-additionalspec">`additionalSpec`</a> (`string: null`) - Additional Service spec values. This should be a multi-line string mapping directly to a Kubernetes `Service` object.

* <a name="v-connectinject" href="#v-connectinject">`connectInject`</a> - Values that configure running the [Connect injector](/docs/platform/k8s/connect.html).

  * <a name="v-connectinject-enabled" href="#v-connectinject-enabled">`enabled`</a> (`boolean: false`) - If true, the chart will install all the resources necessary for the Connect injector process to run. This will enable the injector but will require pods to opt-in with an annotation by default.

  * <a name="v-connectinject-image" href="#v-connectinject-image">`image`</a> (`string: global.imageK8S`) - The name of the Docker image (including any tag) for the [consul-k8s](https://github.com/hashicorp/consul-k8s) binary.

  * <a name="v-connectinject-default" href="#v-connectinject-default">`default`</a> (`boolean: false`) - If true, the injector will inject the Connect sidecar into all pods by default. Otherwise, pods must specify the. [injection annotation](/docs/platform/k8s/connect.html#consul-hashicorp-com-connect-inject) to opt-in to Connect injection. If this is true, pods can use the same annotation to explicitly opt-out of injection.

  * <a name="v-connectinject-imageConsul" href="#v-connectinject-imageConsul">`imageConsul`</a> (`string: global.image`) - The name of the Docker image (including any tag) for Consul. This is used for proxy service registration, Envoy configuration, etc.

  * <a name="v-connectinject-imageEnvoy" href="#v-connectinject-imageEnvoy">`imageEnvoy`</a> (`string: ""`) - The name of the Docker image (including any tag) for the Envoy sidecar. `envoy` must be on the executable path within this image. This Envoy version must be compatible with the Consul version used by the injector. This defaults to letting the injector choose the Envoy image, which is usually `envoyproxy/envoy-alpine`.

  * <a name="v-connectinject-namespaceselector" href="#v-connectinject-namespaceselector">`namespaceSelector`</a> (`string: ""`) - A [selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) for restricting injection to only matching namespaces. By default all namespaces except `kube-system` and `kube-public` will have injection enabled.
    
        ```yaml
        namespaceSelector: |
          matchLabels:
            namespace-label: label-value
        ```

  * <a name="v-connectinject-k8sallownamespaces" href="#v-connectinject-k8sallownamespaces">`k8sAllowNamespaces`</a> - list of k8s namespaces to allow Connect sidecar injection in. If a k8s namespace is not included or is listed in `k8sDenyNamespaces`, pods in that k8s namespace will not be injected even if they are explicitly annotated. Use `["*"]` to automatically allow all k8s namespaces.
  
        For example, `["namespace1", "namespace2"]` will only allow pods in the k8s namespaces `namespace1` and `namespace2` to have Connect sidecars injected and registered with Consul. All other k8s namespaces will be ignored. 
        
        Note: `k8sDenyNamespaces` takes precedence over values defined here and `namespaceSelector` takes precedence over both since it is applied first. `kube-system` and `kube-public` are never injected, even if included here. Requires consul-k8s v0.12+
        
  * <a name="v-connectinject-k8sdenynamespaces" href="#v-connectinject-k8sdenynamespaces">`k8sDenyNamespaces`</a> - list of k8s namespaces that should not allow Connect sidecar injection. This list takes precedence over `k8sAllowNamespaces`. `*` is not supported because then nothing would be allowed to be injected.
  
        For example, if `k8sAllowNamespaces` is `["*"]` and `k8sDenyNamespaces` is `["namespace1", "namespace2"]`, then all k8s namespaces besides `namespace1` and `namespace2` will be injected.
        
        Note: `namespaceSelector` takes precedence over this since it is applied first. `kube-system` and `kube-public` are never injected. Requires consul-k8s v0.12+.
  
  * <a name="v-connectinject-consulnamespaces" href="#v-connectinject-consulnamespaces">`consulNamespaces`</a> - [Enterprise Only] These settings manage the connect injector's interaction with Consul namespaces (requires consul-ent v1.7+ and consul-k8s v0.12+). Also, `global.enableConsulNamespaces` must be true.
  
      * <a name="v-connectinject-consulnamespaces-consuldestinationnamespace" href="#v-connectinject-consulnamespaces-consuldestinationnamespace">`consulDestinationNamespace`</a> (`string: "default"`) - Name of the Consul namespace to register all k8s services into. If the Consul namespace does not already exist, it will be created. This will be ignored if `mirroringK8S` is true.
      
      * <a name="v-connectinject-consulnamespaces-mirroringk8s" href="#v-connectinject-consulnamespaces-mirroringk8s">`mirroringK8S`</a> (`bool: false`) - causes k8s services to be registered into a Consul namespace of the same name as their k8s namespace, optionally prefixed if `mirroringK8SPrefix` is set below. If the Consul namespace does not already exist, it will be created. Turning this on overrides the `consulDestinationNamespace` setting.
      
      * <a name="v-connectinject-consulnamespaces-mirroringk8sprefix" href="#v-connectinject-consulnamespaces-mirroringk8sprefix">`mirroringK8SPrefix`</a> (`string: ""`) - If `mirroringK8S` is set to true, `mirroringK8SPrefix` allows each Consul namespace to be given a prefix. For example, if `mirroringK8SPrefix` is set to `"k8s-"`, a service in the k8s `staging` namespace will be registered into the `k8s-staging` Consul namespace.
      
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

  * <a name="v-connectinject-nodeselector" href="#v-connectinject-nodeselector">`nodeSelector`</a> (`string: null`) - This value defines [`nodeSelector`](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector) labels for `connectInject` pod assignment, formatted as a multi-line string.

        ```yaml
        nodeSelector: |
          beta.kubernetes.io/arch: amd64
        ```

  * <a name="v-connectinject-acl-bindingrule-selector" href="#v-connectinject-acl-bindingrule-selector">`aclBindingRuleSelector`</a> (`string: "serviceaccount.name!=default"`) -
  A [selector](/docs/acl/acl-auth-methods.html#binding-rules) for restricting automatic injection to only matching services based on
  their associated service account. By default, services using the `default` Kubernetes service account will be prevented from logging in.
  This only has effect if ACLs are enabled. Requires Consul 1.5+ and consul-k8s 0.8.0+.

  * <a name="v-connectinject-overrideauthmethodname" href="#v-connectinject-overrideauthmethodname">`overrideAuthMethodName`</a> (`string: ""`) - If not using `global.acls.manageSystemACLs` and instead manually setting up an auth method for Connect inject, set this to the name of your Auth method.
  
  * <a name="v-connectinject-aclinjecttoken" href="#v-connectinject-aclinjecttoken">`aclInjectToken`</a> - Refers to a Kubernetes secret that you have created that contains an ACL token for your Consul cluster which allows the Connect injector the correct permissions. This is only needed if Consul namespaces and ACLs are enabled on the Consul cluster and you are not setting `global.acls.manageSystemACLs` to `true`. This token needs to have `operator = "write"` privileges so that it can create namespaces.
  
      - <a name="v-connectinject-aclinjecttoken-secretname" href="#v-connectinject-aclinjecttoken-secretname">secretName </a>`(string: null)` - The name of the Kubernetes secret.
    
      - <a name="v-connectinject-aclinjecttoken-secretkey" href="#v-connectinject-aclinjecttoken-secretkey">secretKey </a>`(string: null)` - The key within the Kubernetes secret that holds the acl token.

  * <a name="v-connectinject-centralconfig" href="#v-connectinject-centralconfig">`centralConfig`</a> - Values that configure
  Consul's [central configuration](/docs/agent/config_entries.html) feature (requires Consul v1.5+ and consul-k8s v0.8.1+).

      - <a name="v-connectinject-centralconfig-enabled" href="#v-connectinject-centralconfig-enabled">`enabled`</a> (`boolean: true`) -
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

* <a name="v-tests" href="#v-tests">`tests`</a> - Control whether to enable a test for this Helm chart.

  * <a name="v-tests-enabled" href="#v-tests-enabled">`enabled`</a> (`boolean: true`) If true, the test Pod manifest will be generated to be used as a Helm test.
  The pod will be created when a `helm test` command is executed.

## Helm Chart Examples

The below `config.yaml` results in a single server Consul cluster with a `LoadBalancer` to allow external access to the UI and API.

```yaml
# config.yaml
server:
  replicas: 1
  bootstrapExpect: 1

ui:
  service:
    type: LoadBalancer
```

The below `config.yaml` results in a three server Consul Enterprise cluster with 100GB of storage and automatic Connect injection.

Note, this would require a secret that contains the enterprise license key.

```yaml
# config.yaml
global:
  image: "hashicorp/consul-enterprise:1.4.2-ent"

server:
  replicas: 3
  bootstrapExpect: 3
  enterpriseLicense:
    secretName: "consul-license"
    secretKey: "key"
  storage: 100Gi
  connect: true

client:
  grpc: true

connectInject:
  enabled: true
  default: false
```

## Customizing the Helm Chart

Consul within Kubernetes is highly configurable and the Helm chart contains dozens
of the most commonly used configuration options.
If you need to extend the Helm chart with additional options, we recommend using a third-party tool,
such as [kustomize](https://github.com/kubernetes-sigs/kustomize) or [ship](https://github.com/replicatedhq/ship).
Note that the Helm chart heavily relies on Helm lifecycle hooks, and so features like bootstrapping ACLs or TLS
will not work as expected. Additionally, we can make changes to the internal implementation (e.g., renaming template files) that
may be backward incompatible with such customizations.
