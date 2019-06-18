---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-connect"
description: |-
  The 
---

# Connect 

* <a name="connect"></a><a href="#connect">`connect`</a>
    This object allows setting options for the Connect feature.

    The following sub-keys are available:

    * <a name="connect_enabled"></a><a href="#connect_enabled">`enabled`</a> Controls whether
      Connect features are enabled on this agent. Should be enabled on all clients and
      servers in the cluster in order for Connect to function properly. Defaults to false.

    * <a name="connect_ca_provider"></a><a href="#connect_ca_provider">`ca_provider`</a> Controls
      which CA provider to use for Connect's CA. Currently only the `consul` and `vault` providers
      are supported. This is only used when initially bootstrapping the cluster. For an existing
      cluster, use the [Update CA Configuration Endpoint](/api/connect/ca.html#update-ca-configuration).

    * <a name="connect_ca_config"></a><a href="#connect_ca_config">`ca_config`</a> An object which
      allows setting different config options based on the CA provider chosen. This is only
      used when initially bootstrapping the cluster. For an existing cluster, use the [Update CA
      Configuration Endpoint](/api/connect/ca.html#update-ca-configuration).

        The following providers are supported:

        #### Consul CA Provider (`ca_provider = "consul"`)

        * <a name="consul_ca_private_key"></a><a href="#consul_ca_private_key">`private_key`</a> The
        PEM contents of the private key to use for the CA.

        * <a name="consul_ca_root_cert"></a><a href="#consul_ca_root_cert">`root_cert`</a> The
        PEM contents of the root certificate to use for the CA.

        #### Vault CA Provider (`ca_provider = "vault"`)

        * <a name="vault_ca_address"></a><a href="#vault_ca_address">`address`</a> The address of the Vault
        server to connect to.

        * <a name="vault_ca_token"></a><a href="#vault_ca_token">`token`</a> The Vault token to use.

        * <a name="vault_ca_root_pki"></a><a href="#vault_ca_root_pki">`root_pki_path`</a> The
        path to use for the root CA pki backend in Vault. This can be an existing backend with a CA already
        configured, or a blank/unmounted backend in which case Connect will automatically mount/generate the CA.
        The Vault token given above must have `sudo` access to this backend, as well as permission to mount
        the backend at this path if it is not already mounted.

        * <a name="vault_ca_intermediate_pki"></a><a href="#vault_ca_intermediate_pki">`intermediate_pki_path`</a>
        The path to use for the temporary intermediate CA pki backend in Vault. *Connect will overwrite any data
        at this path in order to generate a temporary intermediate CA*. The Vault token given above must have
        `write` access to this backend, as well as permission to mount the backend at this path if it is not
        already mounted.

        #### Common CA Config Options

        <p>There are also a number of common configuration options supported by all providers:</p>

        * <a name="ca_leaf_cert_ttl"></a><a href="#ca_leaf_cert_ttl">`leaf_cert_ttl`</a> The upper bound on the
          lease duration of a leaf certificate issued for a service. In most
          cases a new leaf certificate will be requested by a proxy before this
          limit is reached. This is also the effective limit on how long a
          server outage can last (with no leader) before network connections
          will start being rejected, and as a result the defaults is `72h` to
          last through a weekend without intervention. This value cannot be
          lower than 1 hour or higher than 1 year.

            This value is also used when rotating out old root certificates from
            the cluster. When a root certificate has been inactive (rotated out)
            for more than twice the *current* `leaf_cert_ttl`, it will be removed
            from the trusted list.

        * <a name="ca_csr_max_per_second"></a><a
          href="#ca_csr_max_per_second">`csr_max_per_second`</a> Sets a rate
          limit on the maximum number of Certificate Signing Requests (CSRs) the
          servers will accept. This is used to prevent CA rotation from causing
          unbounded CPU usage on servers. It defaults to 50 which is
          conservative - a 2017 Macbook can process about 100 per second using
          only ~40% of one CPU core - but sufficient for deployments up to ~1500
          service instances before the time it takes to rotate is impacted. For
          larger deployments we recommend increasing this based on the expected
          number of server instances and server resources, or use
          `csr_max_concurrent` instead if servers have more than one core.
          Setting this to zero disables rate limiting. Added in 1.4.1.

        * <a name="ca_csr_max_concurrent"></a><a
          href="#ca_csr_max_concurrent">`csr_max_concurrent`</a> Sets a limit
          on how many Certificate Signing Requests will be processed
          concurrently. Defaults to 0 (disabled). This is useful when you have
          more than one or two cores available to the server. For example on an
          8 core server, setting this to 1 will ensure that even during a CA
          rotation no more than one server core on the leader will be consumed
          at a time with generating new certificates. Setting this is
          recommended _instead_ of `csr_max_per_second` where you know there are
          multiple cores available since it is simpler to reason about limiting
          CSR resources this way without artificially slowing down rotations.
          Added in 1.4.1.

        * <a name="connect_proxy"></a><a href="#connect_proxy">`proxy`</a>
          [**Deprecated**](/docs/connect/proxies/managed-deprecated.html) This
          object allows setting options for the Connect proxies. The following
          sub-keys are available:
          * <a name="connect_proxy_allow_managed_registration"></a><a
            href="#connect_proxy_allow_managed_registration">`allow_managed_api_registration`</a>
            [**Deprecated**](/docs/connect/proxies/managed-deprecated.html)
            Allows managed proxies to be configured with services that are
            registered via the Agent HTTP API. Enabling this would allow anyone
            with permission to register a service to define a command to execute
            for the proxy. By default, this is false to protect against
            arbitrary process execution.
          * <a name="connect_proxy_allow_managed_root"></a><a
            href="#connect_proxy_allow_managed_root">`allow_managed_root`</a>
            [**Deprecated**](/docs/connect/proxies/managed-deprecated.html)
            Allows Consul to start managed proxies if Consul is running as root
            (EUID of the process is zero). We recommend running Consul as a
            non-root user. By default, this is false to protect inadvertently
            running external processes as root.
        * <a name="connect_proxy_defaults"></a><a
          href="#connect_proxy_defaults">`proxy_defaults`</a>
          [**Deprecated**](/docs/connect/proxies/managed-deprecated.html) This
          object configures the default proxy settings for service definitions
          with [managed proxies](/docs/connect/proxies/managed-deprecated.html)
          (now deprecated). It accepts the fields `exec_mode`, `daemon_command`,
          and `config`. These are used as default values for the respective
          fields in the service definition.