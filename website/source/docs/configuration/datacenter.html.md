---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-configuration-datacenter"
description: |-
  The 
---

# Datacenter Options

These options allow you to configure datacenter-wide behavior including communication, . All options apply to both servers and clients unless otherwise noted. 


*   <a name="addresses"></a><a href="#addresses">`addresses`</a> - This is a nested object that allows
    setting bind addresses. In Consul 1.0 and later these can be set to a space-separated list of
    addresses to bind to, or a [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
    template that can potentially resolve to multiple addresses.

    `http`, `https` and `grpc` all support binding to a Unix domain socket. A
    socket can be specified in the form `unix:///path/to/socket`. A new domain
    socket will be created at the given path. If the specified file path already
    exists, Consul will attempt to clear the file and create the domain socket
    in its place. The permissions of the socket file are tunable via the
    [`unix_sockets` config construct](#unix_sockets).

    When running Consul agent commands against Unix socket interfaces, use the
    `-http-addr` argument to specify the path to the socket. You can also place
    the desired values in the `CONSUL_HTTP_ADDR` environment variable.

    For TCP addresses, the environment variable value should be an IP address
    _with the port_. For example: `10.0.0.1:8500` and not `10.0.0.1`. However,
    ports are set separately in the <a href="#ports">`ports`</a> structure when
    defining them in a configuration file.

    The following keys are valid:
    - `dns` - The DNS server. Defaults to `client_addr`
    - `http` - The HTTP API. Defaults to `client_addr`
    - `https` - The HTTPS API. Defaults to `client_addr`
    - `grpc` - The gRPC API. Defaults to `client_addr`

* <a name="advertise_addr"></a><a href="#advertise_addr">`advertise_addr`</a> Equivalent to the `-advertise` command-line flag. The
  advertise address is used to change the address that we advertise to other
  nodes in the cluster. By default, the [`-bind`](#_bind) address is advertised.
  However, in some cases, there may be a routable address that cannot be bound.
  This flag enables gossiping a different address to support this. If this
  address is not routable, the node will be in a constant flapping state as
  other nodes will treat the non-routability as a failure. In Consul 1.0 and
  later this can be set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template.

* <a name="bind_addr"></a><a href="#bind_addr">`bind_addr`</a> Equivalent to the
  `-bind` command-line flag. The address that should be bound to
  for internal cluster communications.
  This is an IP address that should be reachable by all other nodes in the cluster.
  By default, this is "0.0.0.0", meaning Consul will bind to all addresses on
  the local machine and will [advertise](/docs/agent/options.html#_advertise)
  the first available private IPv4 address to the rest of the cluster. If there
  are **multiple private IPv4 addresses** available, Consul will exit with an error at startup. If you specify "[::]", Consul will
  [advertise](/docs/agent/options.html#_advertise) the first available public
  IPv6 address. If there are **multiple public IPv6 addresses** available, Consul
  will exit with an error at startup.
  Consul uses both TCP and UDP and the same port for both. If you
  have any firewalls, be sure to allow both protocols. **In Consul 1.0 and later this can be set to a [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template) template that needs to resolve to a single address.** Some example templates:

    ```sh
    # Using address within a specific CIDR
    $ consul agent -bind '{{ GetPrivateInterfaces | include "network" "10.0.0.0/8" | attr "address" }}'
    ```

    ```sh
    # Using a static network interface name
    $ consul agent -bind '{{ GetInterfaceIP "eth0" }}'
    ```

    ```sh
    # Using regular expression matching for network interface name that is forwardable and up
    $ consul agent -bind '{{ GetAllInterfaces | include "name" "^eth" | include "flags" "forwardable|up" | attr "address" }}'
    ```

* <a name="ca_file"></a><a href="#ca_file">`ca_file`</a> This provides a file path to a PEM-encoded
  certificate authority. The certificate authority is used to check the authenticity of client and
  server connections with the appropriate [`verify_incoming`](#verify_incoming) or
  [`verify_outgoing`](#verify_outgoing) flags.

* <a name="ca_path"></a><a href="#ca_path">`ca_path`</a> This provides a path to a directory of PEM-encoded
  certificate authority files. These certificate authorities are used to check the authenticity of client and
  server connections with the appropriate [`verify_incoming`](#verify_incoming) or
  [`verify_outgoing`](#verify_outgoing) flags.

* <a name="cert_file"></a><a href="#cert_file">`cert_file`</a> This provides a file path to a
  PEM-encoded certificate. The certificate is provided to clients or servers to verify the agent's
  authenticity. It must be provided along with [`key_file`](#key_file).

* <a name="check_update_interval"></a><a href="#check_update_interval">`check_update_interval`</a>
  This interval controls how often check output from
  checks in a steady state is synchronized with the server. By default, this is
  set to 5 minutes ("5m"). Many checks which are in a steady state produce
  slightly different output per run (timestamps, etc) which cause constant writes.
  This configuration allows deferring the sync of check output for a given interval to
  reduce write pressure. If a check ever changes state, the new state and associated
  output is synchronized immediately. To disable this behavior, set the value to "0s".

* <a name="_config_file"></a><a href="#_config_file">`-config-file`</a> - **Command-line flag only** A configuration file
  to load. For more information on
  the format of this file, read the [Configuration Files](#configuration_files) section.
  This option can be specified multiple times to load multiple configuration
  files. If it is specified multiple times, configuration files loaded later
  will merge with configuration files loaded earlier. During a config merge,
  single-value keys (string, int, bool) will simply have their values replaced
  while list types will be appended together.

* <a name="_config_dir"></a><a href="#config_dir">`-config-dir`</a> - **Command-line flag only** A directory of
  configuration files to load. Consul will
  load all files in this directory with the suffix ".json" or ".hcl". The load order
  is alphabetical, and the the same merge routine is used as with the
  [`config-file`](#_config_file) option above. This option can be specified multiple times
  to load multiple directories. Sub-directories of the config directory are not loaded.
  For more information on the format of the configuration files, see the
  [Configuration Files](#configuration_files) section.

* <a name="_config_format"></a><a href="#_config_format">`-config-format`</a> - **Command-line flag only** The format
  of the configuration files to load. Normally, Consul detects the format of the
  config files from the ".json" or ".hcl" extension. Setting this option to
  either "json" or "hcl" forces Consul to interpret any file with or without
  extension to be interpreted in that format.

* <a name="config_entries"></a><a href="#config_entries">`config_entries`</a>
    This object allows setting options for centralized config entries.

    The following sub-keys are available:

    * <a name="config_entries_bootstrap"></a><a href="#config_entries_bootstrap">`bootstrap`</a>
        This is a list of inlined config entries to insert into the state store when the Consul server
        gains leadership. This option is only applicable to server nodes. Each bootstrap
        entry will be created only if it does not exist. When reloading, any new entries
        that have been added to the configuration will be processed. See the
        [configuration entry docs](/docs/agent/config_entries.html) for more details about the
        contents of each entry.

* <a name="data_dir"></a><a href="#data_dir">`data_dir`</a> Equivalent to the
  `-data-dir` command-line flag. This option provides a data directory for the agent to store state. This is required for
  all agents. The directory should be durable across reboots. This is especially
  critical for agents that are running in server mode as they must be able to
  persist cluster state. Additionally, the directory must support the use of
  filesystem locking, meaning some types of mounted folders (e.g. VirtualBox
  shared folders) may not be suitable. **Note:** both server and non-server
  agents may store ACL tokens in the state in this directory so read access may
  grant access to any tokens on servers and to any tokens used during service
  registration on non-servers. On Unix-based platforms the files are written
  with 0600 permissions so you should ensure only trusted processes can execute
  as the same user as Consul. On Windows, you should ensure the directory has
  suitable permissions configured as these will be inherited.

* <a name="datacenter"></a><a href="#datacenter">`datacenter`</a> Equivalent to the
  `-datacenter` command-line flag. This option controls the datacenter in
  which the agent is running. If not provided,
  it defaults to "dc1". Consul has first-class support for multiple datacenters, but
  it relies on proper configuration. Nodes in the same datacenter should be on a single
  LAN.

* <a name="disable_anonymous_signature"></a><a href="#disable_anonymous_signature">
  `disable_anonymous_signature`</a> Disables providing an anonymous signature for de-duplication
  with the update check. See [`disable_update_check`](#disable_update_check).

* <a name="disable_host_node_id"></a><a href="#disable_host_node_id">`disable_host_node_id`</a>
  Equivalent to the `-disable-host-node-id`. Setting
  this to true will prevent Consul from using information from the host to generate a deterministic node ID,
  and will instead generate a random node ID which will be persisted in the data directory. This is useful
  when running multiple Consul agents on the same host for testing. This defaults to false in Consul prior
  to version 0.8.5 and in 0.8.5 and later defaults to true, so you must opt-in for host-based IDs. Host-based
  IDs are generated using https://github.com/shirou/gopsutil/tree/master/host, which is shared with HashiCorp's
  [Nomad](https://www.nomadproject.io/), so if you opt-in to host-based IDs then Consul and Nomad will use
  information on the host to automatically assign the same ID in both systems.

* <a name="disable_http_unprintable_char_filter"></a><a href="#disable_http_unprintable_char_filter">`disable_http_unprintable_char_filter`</a>
  Defaults to false. Consul 1.0.3 fixed a potential security vulnerability where
  malicious users could craft KV keys with unprintable chars that would confuse
  operators using the CLI or UI into taking wrong actions. Users who had data
  written in older versions of Consul that did not have this restriction will be
  unable to delete those values by default in 1.0.3 or later. This setting
  enables those users to _temporarily_ disable the filter such that delete
  operations can work on those keys again to get back to a healthy state. It is
  strongly recommended that this filter is not disabled permanently as it
  exposes the original security vulnerability.

* <a name="disable_remote_exec"></a><a href="#disable_remote_exec">`disable_remote_exec`</a>
  Disables support for remote execution. When set to true, the agent will ignore any incoming
  remote exec requests. In versions of Consul prior to 0.8, this defaulted to false. In Consul
  0.8 the default was changed to true, to make remote exec opt-in instead of opt-out.

* <a name="disable_update_check"></a><a href="#disable_update_check">`disable_update_check`</a>
  Disables automatic checking for security bulletins and new version releases. This is disabled in
  Consul Enterprise.

* <a name="discard_check_output"></a><a href="#discard_check_output">`discard_check_output`</a>
  Discards the output of health checks before storing them. This reduces the number of writes
  to the Consul raft log in environments where health checks have volatile output like
  timestamps, process ids, ...

* <a name="discovery_max_stale"></a><a href="#discovery_max_stale">`discovery_max_stale`</a> - Enables
  stale requests for all service discovery HTTP endpoints. This is equivalent to the
  [`max_stale`](#max_stale) configuration for DNS requests. If this value is zero (default), all service
  discovery HTTP endpoints are forwarded to the leader. If this value is greater than zero, any Consul server
  can handle the service discovery request.  If a Consul server is behind the leader by more than `discovery_max_stale`,
  the query will be re-evaluated on the leader to get more up-to-date results. Consul agents also add a new
  `X-Consul-Effective-Consistency` response header which indicates if the agent did a stale read. `discover-max-stale`
  was introduced in Consul 1.0.7 as a way for Consul operators to force stale requests from clients at the agent level,
  and defaults to zero which matches default consistency behavior in earlier Consul versions.

*   <a name="dns_config"></a><a href="#dns_config">`dns_config`</a> This object allows a number
    of sub-keys to be set which can tune how DNS queries are serviced. See this guide on
    [DNS caching](https://learn.hashicorp.com/consul/security-networking/dns-caching) for more detail.

    The following sub-keys are available:

    * <a name="allow_stale"></a><a href="#allow_stale">`allow_stale`</a> - Enables a stale query
      for DNS information. This allows any Consul server, rather than only the leader, to service
      the request. The advantage of this is you get linear read scalability with Consul servers.
      In versions of Consul prior to 0.7, this defaulted to false, meaning all requests are serviced
      by the leader, providing stronger consistency but less throughput and higher latency. In Consul
      0.7 and later, this defaults to true for better utilization of available servers.

    * <a name="max_stale"></a><a href="#max_stale">`max_stale`</a> - When [`allow_stale`](#allow_stale)
      is specified, this is used to limit how stale results are allowed to be. If a Consul server is
      behind the leader by more than `max_stale`, the query will be re-evaluated on the leader to get
      more up-to-date results. Prior to Consul 0.7.1 this defaulted to 5 seconds; in Consul 0.7.1
      and later this defaults to 10 years ("87600h") which effectively allows DNS queries to be answered
      by any server, no matter how stale. In practice, servers are usually only milliseconds behind the
      leader, so this lets Consul continue serving requests in long outage scenarios where no leader can
      be elected.

    * <a name="node_ttl"></a><a href="#node_ttl">`node_ttl`</a> - By default, this is "0s", so all
      node lookups are served with a 0 TTL value. DNS caching for node lookups can be enabled by
      setting this value. This should be specified with the "s" suffix for second or "m" for minute.

    * <a name="service_ttl"></a><a href="#service_ttl">`service_ttl`</a> - This is a sub-object
      which allows for setting a TTL on service lookups with a per-service policy. The "*" wildcard
      service can be used when there is no specific policy available for a service. By default, all
      services are served with a 0 TTL value. DNS caching for service lookups can be enabled by
      setting this value.

    * <a name="enable_truncate"></a><a href="#enable_truncate">`enable_truncate`</a> - If set to
      true, a UDP DNS query that would return more than 3 records, or more than would fit into a valid
      UDP response, will set the truncated flag, indicating to clients that they should re-query
      using TCP to get the full set of records.

    * <a name="only_passing"></a><a href="#only_passing">`only_passing`</a> - If set to true, any
      nodes whose health checks are warning or critical will be excluded from DNS results. If false,
      the default, only nodes whose healthchecks are failing as critical will be excluded. For
      service lookups, the health checks of the node itself, as well as the service-specific checks
      are considered. For example, if a node has a health check that is critical then all services on
      that node will be excluded because they are also considered critical.

    * <a name="recursor_timeout"></a><a href="#recursor_timeout">`recursor_timeout`</a> - Timeout used
      by Consul when recursively querying an upstream DNS server. See <a href="#recursors">`recursors`</a>
      for more details. Default is 2s. This is available in Consul 0.7 and later.

    * <a name="disable_compression"></a><a href="#disable_compression">`disable_compression`</a> - If
      set to true, DNS responses will not be compressed. Compression was added and enabled by default
      in Consul 0.7.

    * <a name="udp_answer_limit"></a><a href="#udp_answer_limit">`udp_answer_limit`</a> - Limit the number of
      resource records contained in the answer section of a UDP-based DNS
      response. This parameter applies only to UDP DNS queries that are less than 512 bytes. This setting is deprecated
      and replaced in Consul 1.0.7 by <a href="#a_record_limit">`a_record_limit`</a>.

    * <a name="a_record_limit"></a><a href="#a_record_limit">`a_record_limit`</a> - Limit the number of
      resource records contained in the answer section of a A, AAAA or ANY DNS response (both TCP and UDP).
      When answering a question, Consul will use the complete list of
      matching hosts, shuffle the list randomly, and then limit the number of
      answers to `a_record_limit` (default: no limit). This limit does not apply to SRV records.

        In environments where [RFC 3484 Section 6](https://tools.ietf.org/html/rfc3484#section-6) Rule 9
      is implemented and enforced (i.e. DNS answers are always sorted and
      therefore never random), clients may need to set this value to `1` to
      preserve the expected randomized distribution behavior (note:
      [RFC 3484](https://tools.ietf.org/html/rfc3484) has been obsoleted by
      [RFC 6724](https://tools.ietf.org/html/rfc6724) and as a result it should
      be increasingly uncommon to need to change this value with modern
      resolvers).

* <a name="domain"></a><a href="#domain">`domain`</a> Equivalent to the
  [`-domain` command-line flag](#_domain).

    * <a name="enable_additional_node_meta_txt"></a><a href="#enable_additional_node_meta_txt">`enable_additional_node_meta_txt`</a> -
      When set to true, Consul will add TXT records for Node metadata into the Additional section of the DNS responses for several
      query types such as SRV queries. When set to false those records are not emitted. This does not impact the behavior of those
      same TXT records when they would be added to the Answer section of the response like when querying with type TXT or ANY. This
      defaults to true.

    * <a name="soa"></a><a href="#soa">`soa`</a> Allow to tune the setting set up in SOA.
      Non specified values fallback to their default values, all values are integers and
      expressed as seconds.
      <br/><br/>
      The following settings are available:

      * <a name="soa_expire"></a><a href="#soa_expire">`expire`</a> -
        Configure SOA Expire duration in seconds, default value is 86400, ie: 24 hours.

      * <a name="soa_min_ttl"></a><a href="#soa_min_ttl">`min_ttl`</a> -
        Configure SOA DNS minimum TTL.
        As explained in [RFC-2308](https://tools.ietf.org/html/rfc2308) this also controls
        negative cache TTL in most implementations. Default value is 0, ie: no minimum
        delay or negative TTL.

      * <a name="soa_refresh"></a><a href="#soa_refresh">`refresh`</a> -
        Configure SOA Refresh duration in seconds, default value is `3600`, ie: 1 hour.

      * <a name="soa_retry"></a><a href="#soa_retry">`retry`</a> -
        Configures the Retry duration expressed in seconds, default value is
        600, ie: 10 minutes.

    * <a name="dns_use_cache"></a><a href="#dns_use_cache">`use_cache`</a> - When set to true, DNS resolution will use the agent cache described
      in [agent caching](/api/features/caching.html). This setting affects all service and prepared queries DNS requests. Implies [`allow_stale`](#allow_stale)

    * <a name="dns_cache_max_age"></a><a href="#dns_cache_max_age">`cache_max_age`</a> - When [use_cache](#dns_use_cache) is enabled, the agent
      will attempt to re-fetch the result from the servers if the cached value is older than this duration. See: [agent caching](/api/features/caching.html).

* <a name="enable_agent_tls_for_checks"></a><a href="#enable_agent_tls_for_checks">`enable_agent_tls_for_checks`</a>
  When set, uses a subset of the agent's TLS configuration (`key_file`, `cert_file`, `ca_file`, `ca_path`, and
  `server_name`) to set up the client for HTTP or gRPC health checks. This allows services requiring 2-way TLS to
  be checked using the agent's credentials. This was added in Consul 1.0.1 and defaults to false.

* <a name="enable_central_service_config"></a><a href="#enable_central_service_config">`enable_central_service_config`</a>
  When set, the Consul agent will look for any centralized service configurations that match a registering service instance.
  If it finds any, the agent will merge the centralized defaults with the service instance configuration. This allows for
  things like service protocol or proxy configuration to be defined centrally and inherited by any
  affected service registrations.

  * <a name="enable_debug"></a><a href="#enable_debug">`enable_debug`</a> When set, enables some
  additional debugging features. Currently, this is only used to access runtime profiling HTTP endpoints, which
  are available with an `operator:read` ACL regardless of the value of `enable_debug`.

* <a name="enable_script_checks"></a><a href="#enable_script_checks">`enable_script_checks`</a> Equivalent to the
  [`-enable-script-checks` command-line flag](#_enable_script_checks).

    ~> **Security Warning:** Enabling script checks in some configurations may
  introduce a remote execution vulnerability which is known to be targeted by
  malware. We strongly recommend `enable_local_script_checks` instead. See [this
  blog post](https://www.hashicorp.com/blog/protecting-consul-from-rce-risk-in-specific-configurations)
  for more details.

* <a name="enable_local_script_checks"></a><a href="#enable_local_script_checks">`enable_local_script_checks`</a> Equivalent to the
  [`-enable-local-script-checks` command-line flag](#_enable_local_script_checks).

* <a name="enable_syslog"></a><a href="#enable_syslog">`enable_syslog`</a> Equivalent to
  the [`-syslog` command-line flag](#_syslog).

* <a name="encrypt"></a><a href="#encrypt">`encrypt`</a> Equivalent to the
  [`-encrypt` command-line flag](#_encrypt).

* <a name="encrypt_verify_incoming"></a><a href="#encrypt_verify_incoming">`encrypt_verify_incoming`</a> -
  This is an optional parameter that can be used to disable enforcing encryption for incoming gossip in order
  to upshift from unencrypted to encrypted gossip on a running cluster. See [this section]
  (/docs/agent/encryption.html#configuring-gossip-encryption-on-an-existing-cluster) for more information.
  Defaults to true.

* <a name="encrypt_verify_outgoing"></a><a href="#encrypt_verify_outgoing">`encrypt_verify_outgoing`</a> -
  This is an optional parameter that can be used to disable enforcing encryption for outgoing gossip in order
  to upshift from unencrypted to encrypted gossip on a running cluster. See [this section]
  (/docs/agent/encryption.html#configuring-gossip-encryption-on-an-existing-cluster) for more information.
  Defaults to true.

* <a name="disable_keyring_file"></a><a href="#disable_keyring_file">`disable_keyring_file`</a> - Equivalent to the
  [`-disable-keyring-file` command-line flag](#_disable_keyring_file).

* <a name="gossip_lan"></a><a href="#gossip_lan">`gossip_lan`</a> - **(Advanced)** This object contains a number of sub-keys
  which can be set to tune the LAN gossip communications. These are only provided for users running especially large
  clusters that need fine tuning and are prepared to spend significant effort correctly tuning them for their
  environment and workload. **Tuning these improperly can cause Consul to fail in unexpected ways**.
  The default values are appropriate in almost all deployments.

  * <a name="gossip_nodes"></a><a href="#gossip_nodes">`gossip_nodes`</a> - The number of random nodes to send
     gossip messages to per gossip_interval. Increasing this number causes the gossip messages to propagate
     across the cluster more quickly at the expense of increased bandwidth. The default is 3.

  * <a name="gossip_interval"></a><a href="#gossip_interval">`gossip_interval`</a> - The interval between sending
    messages that need to be gossiped that haven't been able to piggyback on probing messages. If this is set to
    zero, non-piggyback gossip is disabled. By lowering this value (more frequent) gossip messages are propagated
    across the cluster more quickly at the expense of increased bandwidth. The default is 200ms.

  * <a name="probe_interval"></a><a href="#probe_interval">`probe_interval`</a> - The interval between random node
    probes. Setting this lower (more frequent) will cause the cluster to detect failed nodes more quickly
    at the expense of increased bandwidth usage. The default is 1s.

  * <a name="probe_timeout"></a><a href="#probe_timeout">`probe_timeout`</a> - The timeout to wait for an ack from
    a probed node before assuming it is unhealthy. This should be at least the 99-percentile of RTT (round-trip time) on
    your network. The default is 500ms and is a conservative value suitable for almost all realistic deployments.

  * <a name="retransmit_mult"></a><a href="#retransmit_mult">`retransmit_mult`</a> - The multiplier for the number
    of retransmissions that are attempted for messages broadcasted over gossip. The number of retransmits is scaled
    using this multiplier and the cluster size. The higher the multiplier, the more likely a failed broadcast is to
    converge at the expense of increased bandwidth. The default is 4.

  * <a name="suspicion_mult"></a><a href="#suspicion_mult">`suspicion_mult`</a> - The multiplier for determining the
    time an inaccessible node is considered suspect before declaring it dead. The timeout is scaled with the cluster
    size and the probe_interval. This allows the timeout to scale properly with expected propagation delay with a
    larger cluster size. The higher the multiplier, the longer an inaccessible node is considered part of the
    cluster before declaring it dead, giving that suspect node more time to refute if it is indeed still alive. The
    default is 4.

* <a name="gossip_wan"></a><a href="#gossip_wan">`gossip_wan`</a> - **(Advanced)** This object contains a number of sub-keys
  which can be set to tune the WAN gossip communications. These are only provided for users running especially large
  clusters that need fine tuning and are prepared to spend significant effort correctly tuning them for their
  environment and workload. **Tuning these improperly can cause Consul to fail in unexpected ways**.
  The default values are appropriate in almost all deployments.

    * <a name="gossip_nodes"></a><a href="#gossip_nodes">`gossip_nodes`</a> - The number of random nodes to send
     gossip messages to per gossip_interval. Increasing this number causes the gossip messages to propagate
     across the cluster more quickly at the expense of increased bandwidth. The default is 3.

  * <a name="gossip_interval"></a><a href="#gossip_interval">`gossip_interval`</a> - The interval between sending
    messages that need to be gossiped that haven't been able to piggyback on probing messages. If this is set to
    zero, non-piggyback gossip is disabled. By lowering this value (more frequent) gossip messages are propagated
    across the cluster more quickly at the expense of increased bandwidth. The default is 200ms.

  * <a name="probe_interval"></a><a href="#probe_interval">`probe_interval`</a> - The interval between random node
    probes. Setting this lower (more frequent) will cause the cluster to detect failed nodes more quickly
    at the expense of increased bandwidth usage. The default is 1s.

  * <a name="probe_timeout"></a><a href="#probe_timeout">`probe_timeout`</a> - The timeout to wait for an ack from
    a probed node before assuming it is unhealthy. This should be at least the 99-percentile of RTT (round-trip time) on
    your network. The default is 500ms and is a conservative value suitable for almost all realistic deployments.

  * <a name="retransmit_mult"></a><a href="#retransmit_mult">`retransmit_mult`</a> - The multiplier for the number
    of retransmissions that are attempted for messages broadcasted over gossip. The number of retransmits is scaled
    using this multiplier and the cluster size. The higher the multiplier, the more likely a failed broadcast is to
    converge at the expense of increased bandwidth. The default is 4.

  * <a name="suspicion_mult"></a><a href="#suspicion_mult">`suspicion_mult`</a> - The multiplier for determining the
    time an inaccessible node is considered suspect before declaring it dead. The timeout is scaled with the cluster
    size and the probe_interval. This allows the timeout to scale properly with expected propagation delay with a
    larger cluster size. The higher the multiplier, the longer an inaccessible node is considered part of the
    cluster before declaring it dead, giving that suspect node more time to refute if it is indeed still alive. The
    default is 4.

* <a name="key_file"></a><a href="#key_file">`key_file`</a> This provides a the file path to a
  PEM-encoded private key. The key is used with the certificate to verify the agent's authenticity.
  This must be provided along with [`cert_file`](#cert_file).

*   <a name="http_config"></a><a href="#http_config">`http_config`</a>
    This object allows setting options for the HTTP API.

    The following sub-keys are available:

    * <a name="block_endpoints"></a><a href="#block_endpoints">`block_endpoints`</a>
      This object is a list of HTTP API endpoint prefixes to block on the agent, and defaults to
      an empty list, meaning all endpoints are enabled. Any endpoint that has a common prefix
      with one of the entries on this list will be blocked and will return a 403 response code
      when accessed. For example, to block all of the V1 ACL endpoints, set this to
      `["/v1/acl"]`, which will block `/v1/acl/create`, `/v1/acl/update`, and the other ACL
      endpoints that begin with `/v1/acl`. This only works with API endpoints, not `/ui` or
      `/debug`, those must be disabled with their respective configuration options. Any CLI
      commands that use disabled endpoints will no longer function as well. For more general
      access control, Consul's [ACL system](https://learn.hashicorp.com/consul/security-networking/production-acls) should be used, but this option
      is useful for removing access to HTTP API endpoints completely, or on specific agents. This
      is available in Consul 0.9.0 and later.

    * <a name="response_headers"></a><a href="#response_headers">`response_headers`</a>
      This object allows adding headers to the HTTP API responses.
      For example, the following config can be used to enable
      [CORS](https://en.wikipedia.org/wiki/Cross-origin_resource_sharing) on
      the HTTP API endpoints:

          ```javascript
            {
              "http_config": {
                "response_headers": {
                  "Access-Control-Allow-Origin": "*"
                }
              }
            }
          ```
    * <a name="allow_write_http_from"></a><a href="#allow_write_http_from">`allow_write_http_from`</a>
      This object is a list of networks in CIDR notation (eg "127.0.0.0/8") that are allowed
      to call the agent write endpoints. It defaults to an empty list, which means all networks
      are allowed.
      This is used to make the agent read-only, except for select ip ranges.
      * To block write calls from anywhere, use `[ "255.255.255.255/32" ]`.
      * To only allow write calls from localhost, use `[ "127.0.0.0/8" ]`
      * To only allow specific IPs, use `[ "10.0.0.1/32", "10.0.0.2/32" ]`

* <a name="leave_on_terminate"></a><a href="#leave_on_terminate">`leave_on_terminate`</a> If
  enabled, when the agent receives a TERM signal, it will send a `Leave` message to the rest
  of the cluster and gracefully leave. The default behavior for this feature varies based on
  whether or not the agent is running as a client or a server (prior to Consul 0.7 the default
  value was unconditionally set to `false`). On agents in client-mode, this defaults to `true`
  and for agents in server-mode, this defaults to `false`.

* <a name="log_file"></a><a href="#log_file">`log_file`</a> Equivalent to the
  [`-log-file` command-line flag](#_log_file).

* <a name="log_level"></a><a href="#log_level">`log_level`</a> Equivalent to the
  [`-log-level` command-line flag](#_log_level).

* <a name="node_id"></a><a href="#node_id">`node_id`</a> Equivalent to the
  [`-node-id` command-line flag](#_node_id).

* <a name="node_name"></a><a href="#node_name">`node_name`</a> Equivalent to the
  [`-node` command-line flag](#_node).

* <a name="node_meta"></a><a href="#node_meta">`node_meta`</a> Available in Consul 0.7.3 and later,
  This object allows associating arbitrary metadata key/value pairs with the local node, which can
  then be used for filtering results from certain catalog endpoints. See the
  [`-node-meta` command-line flag](#_node_meta) for more information.

    ```javascript
      {
        "node_meta": {
            "instance_type": "t2.medium"
        }
      }
    ```

* <a name="ports"></a><a href="#ports">`ports`</a> This is a nested object that allows setting
  the bind ports for the following keys:
    * <a name="dns_port"></a><a href="#dns_port">`dns`</a> - The DNS server, -1 to disable. Default 8600. TCP and UDP.
    * <a name="http_port"></a><a href="#http_port">`http`</a> - The HTTP API, -1 to disable. Default 8500. TCP only.
    * <a name="https_port"></a><a href="#https_port">`https`</a> - The HTTPS
      API, -1 to disable. Default -1 (disabled). **We recommend using `8501`** for
      `https` by convention as some tooling will work automatically with this.
    * <a name="grpc_port"></a><a href="#grpc_port">`grpc`</a> - The gRPC API, -1
      to disable. Default -1 (disabled). **We recommend using `8502`** for
      `grpc` by convention as some tooling will work automatically with this.
      This is set to `8502` by default when the agent runs in `-dev` mode.
      Currently gRPC is only used to expose Envoy xDS API to Envoy proxies.
    * <a name="serf_lan_port"></a><a href="#serf_lan_port">`serf_lan`</a> - The Serf LAN port. Default 8301. TCP and UDP.
    * <a name="serf_wan_port"></a><a href="#serf_wan_port">`serf_wan`</a> - The Serf WAN port. Default 8302. Set to -1
      to disable. **Note**: this will disable WAN federation which is not recommended. Various catalog and WAN related
      endpoints will return errors or empty results. TCP and UDP.
    * <a name="server_rpc_port"></a><a href="#server_rpc_port">`server`</a> - Server RPC address. Default 8300. TCP only.
    * <a name="proxy_min_port"></a><a href="#proxy_min_port">`proxy_min_port`</a> [**Deprecated**](/docs/connect/proxies/managed-deprecated.html) - Minimum port number to use for automatically assigned [managed proxies](/docs/connect/proxies/managed-deprecated.html). Default 20000.
    * <a name="proxy_max_port"></a><a href="#proxy_max_port">`proxy_max_port`</a> [**Deprecated**](/docs/connect/proxies/managed-deprecated.html) - Maximum port number to use for automatically assigned [managed proxies](/docs/connect/proxies/managed-deprecated.html). Default 20255.
    * <a name="sidecar_min_port"></a><a
      href="#sidecar_min_port">`sidecar_min_port`</a> - Inclusive minimum port
      number to use for automatically assigned [sidecar service
      registrations](/docs/connect/registration/sidecar-service.html). Default 21000.
      Set to `0` to disable automatic port assignment.
    * <a name="sidecar_max_port"></a><a
      href="#sidecar_max_port">`sidecar_max_port`</a> - Inclusive maximum port
      number to use for automatically assigned [sidecar service
      registrations](/docs/connect/registration/sidecar-service.html). Default 21255.
      Set to `0` to disable automatic port assignment.

* <a name="protocol"></a><a href="#protocol">`protocol`</a> Equivalent to the
  [`-protocol` command-line flag](#_protocol).

* <a name="reap"></a><a href="#reap">`reap`</a> This controls Consul's automatic reaping of child processes,
  which is useful if Consul is running as PID 1 in a Docker container. If this isn't specified, then Consul will
  automatically reap child processes if it detects it is running as PID 1. If this is set to true or false, then
  it controls reaping regardless of Consul's PID (forces reaping on or off, respectively). This option was removed
  in Consul 0.7.1. For later versions of Consul, you will need to reap processes using a wrapper, please see the
  [Consul Docker image entry point script](https://github.com/hashicorp/docker-consul/blob/master/0.X/docker-entrypoint.sh)
  for an example. If you are using Docker 1.13.0 or later, you can use the new `--init` option of the `docker run` command
  and docker will enable an init process with PID 1 that reaps child processes for the container.
  More info on [Docker docs](https://docs.docker.com/engine/reference/commandline/run/#options).

* <a name="reconnect_timeout"></a><a href="#reconnect_timeout">`reconnect_timeout`</a> This controls
  how long it takes for a failed node to be completely removed from the cluster. This defaults to
  72 hours and it is recommended that this is set to at least double the maximum expected recoverable
  outage time for a node or network partition. WARNING: Setting this time too low could cause Consul
  servers to be removed from quorum during an extended node failure or partition, which could complicate
  recovery of the cluster. The value is a time with a unit suffix, which can be "s", "m", "h" for seconds,
  minutes, or hours. The value must be >= 8 hours.

* <a name="recursors"></a><a href="#recursors">`recursors`</a> This flag provides addresses of
  upstream DNS servers that are used to recursively resolve queries if they are not inside the service
  domain for Consul. For example, a node can use Consul directly as a DNS server, and if the record is
  outside of the "consul." domain, the query will be resolved upstream. As of Consul 1.0.1 recursors
  can be provided as IP addresses or as go-sockaddr templates. IP addresses are resolved in order,
  and duplicates are ignored.

* <a name="rejoin_after_leave"></a><a href="#rejoin_after_leave">`rejoin_after_leave`</a> Equivalent
  to the [`-rejoin` command-line flag](#_rejoin).

* `retry_join` - Equivalent to the [`-retry-join`](#retry-join) command-line flag.

* <a name="retry_interval"></a><a href="#retry_interval">`retry_interval`</a> Equivalent to the
  [`-retry-interval` command-line flag](#_retry_interval).

* <a name="skip_leave_on_interrupt"></a><a
  href="#skip_leave_on_interrupt">`skip_leave_on_interrupt`</a> This is
  similar to [`leave_on_terminate`](#leave_on_terminate) but only affects
  interrupt handling. When Consul receives an interrupt signal (such as
  hitting Control-C in a terminal), Consul will gracefully leave the cluster.
  Setting this to `true` disables that behavior. The default behavior for
  this feature varies based on whether or not the agent is running as a
  client or a server (prior to Consul 0.7 the default value was
  unconditionally set to `false`). On agents in client-mode, this defaults
  to `false` and for agents in server-mode, this defaults to `true`
  (i.e. Ctrl-C on a server will keep the server in the cluster and therefore
  quorum, and Ctrl-C on a client will gracefully leave).

* <a name="serf_lan"></a><a href="#serf_lan_bind">`serf_lan`</a> Equivalent to
  the `-serf-lan-bind` command-line flag. The address that should be bound to for Serf LAN gossip communications. This
  is an IP address that should be reachable by all other LAN nodes in the
  cluster. By default, the value follows the same rules as [`-bind` command-line
  flag](#_bind), and if this is not specified, the `-bind` option is used. This
  is available in Consul 0.7.1 and later. In Consul 1.0 and later this can be
  set to a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
  template

* <a name="start_join"></a><a href="#start_join">`start_join`</a> An array of strings specifying addresses
  of nodes to [`-join`](#_join) upon startup. Note that using
  <a href="#retry_join">`retry_join`</a> could be more appropriate to help
  mitigate node startup race conditions when automating a Consul cluster
  deployment.

* <a name="syslog_facility"></a><a href="#syslog_facility">`syslog_facility`</a> When
  [`enable_syslog`](#enable_syslog) is provided, this controls to which
  facility messages are sent. By default, `LOCAL0` will be used.

* <a name="tls_min_version"></a><a href="#tls_min_version">`tls_min_version`</a> Added in Consul
  0.7.4, this specifies the minimum supported version of TLS. Accepted values are "tls10", "tls11"
  or "tls12". This defaults to "tls12". WARNING: TLS 1.1 and lower are generally considered less
  secure; avoid using these if possible.

* <a name="tls_cipher_suites"></a><a href="#tls_cipher_suites">`tls_cipher_suites`</a> Added in Consul
  0.8.2, this specifies the list of supported ciphersuites as a comma-separated-list. The list of all
  supported ciphersuites is available in the [source code](https://github.com/hashicorp/consul/blob/master/tlsutil/config.go#L363).

* <a name="tls_prefer_server_cipher_suites"></a><a href="#tls_prefer_server_cipher_suites">
  `tls_prefer_server_cipher_suites`</a> Added in Consul 0.8.2, this will cause Consul to prefer the
  server's ciphersuite over the client ciphersuites.

* <a name="ui"></a><a href="#ui">`ui`</a> - Equivalent to the [`-ui`](#_ui)
  command-line flag.

* <a name="ui_dir"></a><a href="#ui_dir">`ui_dir`</a> - Equivalent to the
  [`-ui-dir`](#_ui_dir) command-line flag. This configuration key is not required as of Consul version 0.7.0 and later. Specifying this configuration key will enable the web UI. There is no need to specify both ui-dir and ui. Specifying both will result in an error.

*   <a name="unix_sockets"></a><a href="#unix_sockets">`unix_sockets`</a> - This
    allows tuning the ownership and permissions of the
    Unix domain socket files created by Consul. Domain sockets are only used if
    the HTTP address is configured with the `unix://` prefix.

    It is important to note that this option may have different effects on
    different operating systems. Linux generally observes socket file permissions
    while many BSD variants ignore permissions on the socket file itself. It is
    important to test this feature on your specific distribution. This feature is
    currently not functional on Windows hosts.

    The following options are valid within this construct and apply globally to all
    sockets created by Consul:
    - `user` - The name or ID of the user who will own the socket file.
    - `group` - The group ID ownership of the socket file. This option
      currently only supports numeric IDs.
    - `mode` - The permission bits to set on the file.

* <a name="verify_incoming"></a><a href="#verify_incoming">`verify_incoming`</a> - If
  set to true, Consul requires that all incoming
  connections make use of TLS and that the client provides a certificate signed
  by a Certificate Authority from the [`ca_file`](#ca_file) or [`ca_path`](#ca_path).
  This applies to both server RPC and to the HTTPS API. By default, this is false, and
  Consul will not enforce the use of TLS or verify a client's authenticity.

* <a name="verify_incoming_rpc"></a><a href="#verify_incoming_rpc">`verify_incoming_rpc`</a> - If
  set to true, Consul requires that all incoming RPC
  connections make use of TLS and that the client provides a certificate signed
  by a Certificate Authority from the [`ca_file`](#ca_file) or [`ca_path`](#ca_path). By default,
  this is false, and Consul will not enforce the use of TLS or verify a client's authenticity.

* <a name="verify_incoming_https"></a><a href="#verify_incoming_https">`verify_incoming_https`</a> - If
  set to true, Consul requires that all incoming HTTPS
  connections make use of TLS and that the client provides a certificate signed
  by a Certificate Authority from the [`ca_file`](#ca_file) or [`ca_path`](#ca_path). By default,
  this is false, and Consul will not enforce the use of TLS or verify a client's authenticity. To
  enable the HTTPS API, you must define an HTTPS port via the [`ports`](#ports) configuration. By
  default, HTTPS is disabled.

* <a name="verify_outgoing"></a><a href="#verify_outgoing">`verify_outgoing`</a> - If set to
  true, Consul requires that all outgoing connections from this agent
  make use of TLS and that the server provides a certificate that is signed by
  a Certificate Authority from the [`ca_file`](#ca_file) or [`ca_path`](#ca_path). By default,
  this is false, and Consul will not make use of TLS for outgoing connections. This applies to clients
  and servers as both will make outgoing connections.

    ~> **Security Note:** Note that servers that specify `verify_outgoing =
    true` will always talk to other servers over TLS, but they still _accept_
    non-TLS connections to allow for a transition of all clients to TLS.
    Currently the only way to enforce that no client can communicate with a
    server unencrypted is to also enable `verify_incoming` which requires client
    certificates too.

* <a name="verify_server_hostname"></a><a
  href="#verify_server_hostname">`verify_server_hostname`</a> - If set to true,
  Consul verifies for all outgoing TLS connections that the TLS certificate
  presented by the servers matches "server.&lt;datacenter&gt;.&lt;domain&gt;"
  hostname. By default, this is false, and Consul does not verify the hostname
  of the certificate, only that it is signed by a trusted CA. This setting is
  _critical_ to prevent a compromised client from being restarted as a server
  and having all cluster state _including all ACL tokens and Connect CA root keys_
  replicated to it. This is new in 0.5.1.

    ~> **Security Note:** From versions 0.5.1 to 1.4.0, due to a bug, setting
  this flag alone _does not_ imply `verify_outgoing` and leaves client to server
  and server to server RPCs unencrypted despite the documentation stating otherwise. See
  [CVE-2018-19653](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2018-19653)
  for more details. For those versions you **must also set `verify_outgoing =
  true`** to ensure encrypted RPC connections.

* <a name="watches"></a><a href="#watches">`watches`</a> - Watches is a list of watch
  specifications which allow an external process to be automatically invoked when a
  particular data view is updated. See the
   [watch documentation](/docs/agent/watches.html) for more detail. Watches can be
   modified when the configuration is reloaded.

