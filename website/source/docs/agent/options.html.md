---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-agent-config"
description: |-
  The agent has various configuration options that can be specified via the command-line or via configuration files. All of the configuration options are completely optional. Defaults are specified with their descriptions.
---

# Configuration

The agent has various configuration options that can be specified via
the command-line or via configuration files. All of the configuration
options are completely optional. Defaults are specified with their
descriptions.

Configuration precedence is evaluated in the following order:

1. Command line arguments
2. Environment Variables
3. Configuration files

When loading configuration, Consul loads the configuration from files and
directories in lexical order. For example, configuration file
`basic_config.json` will be processed before `extra_config.json`. Configuration
can be in either [HCL](https://github.com/hashicorp/hcl#syntax) or JSON format.
Available in Consul 1.0 and later, the HCL support now requires an `.hcl` or
`.json` extension on all configuration files in order to specify their format.

Configuration specified later will be merged into configuration specified
earlier. In most cases, "merge" means that the later version will override the
earlier. In some cases, such as event handlers, merging appends the handlers to
the existing configuration. The exact merging behavior is specified for each
option below.

Consul also supports reloading configuration when it receives the
SIGHUP signal. Not all changes are respected, but those that are
are documented below in the
[Reloadable Configuration](#reloadable-configuration) section. The
[reload command](/docs/commands/reload.html) can also be used to trigger a
configuration reload.

## <a name="commandline_options"></a>Command-line Options

The options below are all specified on the command-line.

* <a name="_advertise"></a><a href="#_advertise">`-advertise`</a> - The advertise
  address is used to change the address that we
  advertise to other nodes in the cluster. By default, the [`-bind`](#_bind) address is
  advertised. However, in some cases, there may be a routable address that cannot
  be bound. This flag enables gossiping a different address to support this.
  If this address is not routable, the node will be in a constant flapping state
  as other nodes will treat the non-routability as a failure.

* <a name="_advertise-wan"></a><a href="#_advertise-wan">`-advertise-wan`</a> - The
  advertise WAN address is used to change the address that we advertise to server nodes
  joining through the WAN. This can also be set on client agents when used in combination
  with the <a href="#translate_wan_addrs">`translate_wan_addrs`</a> configuration
  option. By default, the [`-advertise`](#_advertise) address is advertised. However, in some
  cases all members of all datacenters cannot be on the same physical or virtual network,
  especially on hybrid setups mixing cloud and private datacenters. This flag enables server
  nodes gossiping through the public network for the WAN while using private VLANs for gossiping
  to each other and their client agents, and it allows client agents to be reached at this
  address when being accessed from a remote datacenter if the remote datacenter is configured
  with <a href="#translate_wan_addrs">`translate_wan_addrs`</a>.

* <a name="_bootstrap"></a><a href="#_bootstrap">`-bootstrap`</a> - This flag is used to control if a
  server is in "bootstrap" mode. It is important that
  no more than one server *per* datacenter be running in this mode. Technically, a server in bootstrap mode
  is allowed to self-elect as the Raft leader. It is important that only a single node is in this mode;
  otherwise, consistency cannot be guaranteed as multiple nodes are able to self-elect.
  It is not recommended to use this flag after a cluster has been bootstrapped.

* <a name="_bootstrap_expect"></a><a href="#_bootstrap_expect">`-bootstrap-expect`</a> - This flag
  provides the number of expected servers in the datacenter.
  Either this value should not be provided or the value must agree with other servers in
  the cluster. When provided, Consul waits until the specified number of servers are
  available and then bootstraps the cluster. This allows an initial leader to be elected
  automatically. This cannot be used in conjunction with the legacy [`-bootstrap`](#_bootstrap) flag.
  This flag requires [`-server`](#_server) mode.

* <a name="_bind"></a><a href="#_bind">`-bind`</a> - The address that should be bound to
  for internal cluster communications.
  This is an IP address that should be reachable by all other nodes in the cluster.
  By default, this is "0.0.0.0", meaning Consul will bind to all addresses on
the local machine and will [advertise](/docs/agent/options.html#_advertise)
the first available private IPv4 address to the rest of the cluster. If there
are multiple private IPv4 addresses available, Consul will exit with an error
at startup. If you specify "[::]", Consul will
[advertise](/docs/agent/options.html#_advertise) the first available public
IPv6 address. If there are multiple public IPv6 addresses available, Consul
will exit with an error at startup.
  Consul uses both TCP and UDP and the same port for both. If you
  have any firewalls, be sure to allow both protocols.

* <a name="_serf_wan_bind"></a><a href="#_serf_wan_bind">`-serf-wan-bind`</a> - The address that should be bound to for Serf WAN gossip communications.
  By default, the value follows the same rules as [`-bind` command-line flag](#_bind), and if this is not specified, the `-bind` option is used. This
  is available in Consul 0.7.1 and later.

* <a name="_serf_lan_bind"></a><a href="#_serf_lan_bind">`-serf-lan-bind`</a> - The address that should be bound to for Serf LAN gossip communications.
  This is an IP address that should be reachable by all other LAN nodes in the cluster. By default, the value follows the same rules as
  [`-bind` command-line flag](#_bind), and if this is not specified, the `-bind` option is used. This is available in Consul 0.7.1 and later.

* <a name="_client"></a><a href="#_client">`-client`</a> - The address to which
  Consul will bind client interfaces, including the HTTP and DNS servers. By default,
  this is "127.0.0.1", allowing only loopback connections. In Consul 1.0 and later
  this can be set to a space-separated list of addresses to bind to, or a
  [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template) template
  that can potentially resolve to multiple addresses.

* <a name="_config_file"></a><a href="#_config_file">`-config-file`</a> - A configuration file
  to load. For more information on
  the format of this file, read the [Configuration Files](#configuration_files) section.
  This option can be specified multiple times to load multiple configuration
  files. If it is specified multiple times, configuration files loaded later
  will merge with configuration files loaded earlier. During a config merge,
  single-value keys (string, int, bool) will simply have their values replaced
  while list types will be appended together.

* <a name="_config_dir"></a><a href="#_config_dir">`-config-dir`</a> - A directory of
  configuration files to load. Consul will
  load all files in this directory with the suffix ".json". The load order
  is alphabetical, and the the same merge routine is used as with the
  [`config-file`](#_config_file) option above. This option can be specified multiple times
  to load multiple directories. Sub-directories of the config directory are not loaded.
  For more information on the format of the configuration files, see the
  [Configuration Files](#configuration_files) section.

* <a name="_config_format"></a><a href="#_config_format">`-config-format`</a> - The format
  of the configuration files to load. Normally, Consul detects the format of the
  config files from the ".json" or ".hcl" extension. Setting this option to
  either "json" or "hcl" forces Consul to interpret any file with or without
  extension to be interpreted in that format.

* <a name="_data_dir"></a><a href="#_data_dir">`-data-dir`</a> - This flag provides
  a data directory for the agent to store state.
  This is required for all agents. The directory should be durable across reboots.
  This is especially critical for agents that are running in server mode as they
  must be able to persist cluster state. Additionally, the directory must support
  the use of filesystem locking, meaning some types of mounted folders (e.g. VirtualBox
  shared folders) may not be suitable.

* <a name="_datacenter"></a><a href="#_datacenter">`-datacenter`</a> - This flag controls the datacenter in
  which the agent is running. If not provided,
  it defaults to "dc1". Consul has first-class support for multiple datacenters, but
  it relies on proper configuration. Nodes in the same datacenter should be on a single
  LAN.

* <a name="_dev"></a><a href="#_dev">`-dev`</a> - Enable development server
  mode. This is useful for quickly starting a Consul agent with all persistence
  options turned off, enabling an in-memory server which can be used for rapid
  prototyping or developing against the API. This mode is **not** intended for
  production use as it does not write any data to disk.

* <a name="_disable_host_node_id"></a><a href="#_disable_host_node_id">`-disable-host-node-id`</a> - Setting
  this to true will prevent Consul from using information from the host to generate a deterministic node ID,
  and will instead generate a random node ID which will be persisted in the data directory. This is useful
  when running multiple Consul agents on the same host for testing. This defaults to false in Consul prior
  to version 0.8.5 and in 0.8.5 and later defaults to true, so you must opt-in for host-based IDs. Host-based
  IDs are generated using https://github.com/shirou/gopsutil/tree/master/host, which is shared with HashiCorp's
  [Nomad](https://www.nomadproject.io/), so if you opt-in to host-based IDs then Consul and Nomad will use
  information on the host to automatically assign the same ID in both systems.

* <a name="_disable_keyring_file"></a><a href="#_disable_keyring_file">`-disable-keyring-file`</a> - If set,
  the keyring will not be persisted to a file. Any installed keys will be lost on shutdown, and only the given
  `-encrypt` key will be available on startup. This defaults to false.

* <a name="_dns_port"></a><a href="#_dns_port">`-dns-port`</a> - the DNS port to listen on.
  This overrides the default port 8600. This is available in Consul 0.7 and later.

* <a name="_domain"></a><a href="#_domain">`-domain`</a> - By default, Consul responds to DNS queries
  in the "consul." domain. This flag can be used to change that domain. All queries in this domain
  are assumed to be handled by Consul and will not be recursively resolved.

* <a name="_enable_script_checks"></a><a href="#_enable_script_checks">`-enable-script-checks`</a> This
  controls whether [health checks that execute scripts](/docs/agent/checks.html) are enabled on
  this agent, and defaults to `false` so operators must opt-in to allowing these. If enabled,
  it is recommended to [enable ACLs](/docs/guides/acl.html) as well to control which users are
  allowed to register new checks to execute scripts. This was added in Consul 0.9.0.

* <a name="_encrypt"></a><a href="#_encrypt">`-encrypt`</a> - Specifies the secret key to
  use for encryption of Consul
  network traffic. This key must be 16-bytes that are Base64-encoded. The
  easiest way to create an encryption key is to use
  [`consul keygen`](/docs/commands/keygen.html). All
  nodes within a cluster must share the same encryption key to communicate.
  The provided key is automatically persisted to the data directory and loaded
  automatically whenever the agent is restarted. This means that to encrypt
  Consul's gossip protocol, this option only needs to be provided once on each
  agent's initial startup sequence. If it is provided after Consul has been
  initialized with an encryption key, then the provided key is ignored and
  a warning will be displayed.

* <a name="_hcl"></a><a href="#_hcl">`-hcl`</a> - A HCL configuration fragment.
  This HCL configuration fragment is appended to the configuration and allows
  to specify the full range of options of a config file on the command line.
  This option can be specified multiple times. This was added in Consul 1.0.

* <a name="_http_port"></a><a href="#_http_port">`-http-port`</a> - the HTTP API port to listen on.
  This overrides the default port 8500. This option is very useful when deploying Consul
  to an environment which communicates the HTTP port through the environment e.g. PaaS like CloudFoundry, allowing
  you to set the port directly via a Procfile.

* <a name="_join"></a><a href="#_join">`-join`</a> - Address of another agent
  to join upon starting up. This can be
  specified multiple times to specify multiple agents to join. If Consul is
  unable to join with any of the specified addresses, agent startup will
  fail. By default, the agent won't join any nodes when it starts up.
  Note that using
  <a href="#retry_join">`retry_join`</a> could be more appropriate to help
  mitigate node startup race conditions when automating a Consul cluster
  deployment.\

<a name="_retry_join"></a>

* `-retry-join` - Similar to [`-join`](#_join) but allows retrying a join if the
  first attempt fails. This is useful for cases where you know the address will
  eventually be available. The list can contain IPv4, IPv6, or DNS addresses. If
  Consul is running on the non-default Serf LAN port, this must be specified as
  well. IPv6 must use the "bracketed" syntax. If multiple values are given, they
  are tried and retried in the order listed until the first succeeds. Here are
  some examples:

    ```sh
    # Using a DNS entry
    $ consul agent -retry-join "consul.domain.internal"
    ```

    ```sh
    # Using IPv4
    $ consul agent -retry-join "10.0.4.67"
    ```

    ```sh
    # Using IPv6
    $ consul agent -retry-join "[::1]:8301"
    ```

    ```sh
    # Using Cloud Auto-Joining
    $ consul agent -retry-join "provider=aws tag_key=..."
    ```

    ### Cloud Auto-Joining

    As of Consul 0.9.1, `retry-join` accepts a unified interface using the
    [go-discover](https://github.com/hashicorp/go-discover) library for doing
    automatic cluster joining using cloud metadata. To use retry-join with a
    supported cloud provider, specify the configuration on the command line or
    configuration file as a `key=value key=value ...` string.

	In Consul 0.9.1-0.9.3 the values need to be URL encoded but for most
	practical purposes you need to replace spaces with `+` signs.

	As of Consul 1.0 the values are taken literally and must not be URL
	encoded. If the values contain spaces, backslashes or double quotes then
	they need to be double quoted and the usual escaping rules apply.

    ```sh
    $ consul agent -retry-join 'provider=my-cloud config=val config2="some other val" ...'
    ```

    or via a configuration file:

    ```json
    {
      "retry_join": ["provider=my-cloud config=val config2=\"some other val\" ..."]
    }
    ```

    The cloud provider-specific configurations are detailed below. This can be
    combined with static IP or DNS addresses or even multiple configurations
    for different providers.

    In order to use discovery behind a proxy, you will need to set
    `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment variables per
    [Golang `net/http` library](https://golang.org/pkg/net/http/#ProxyFromEnvironment).

    The following sections give the options specific to each supported cloud
    provider.

    ### Amazon EC2

    This returns the first private IP address of all servers in the given
    region which have the given `tag_key` and `tag_value`.

    ```sh
    $ consul agent -retry-join "provider=aws tag_key=... tag_value=..."
    ```

    ```json
    {
      "retry_join": ["provider=aws tag_key=... tag_value=..."]
    }
    ```

    - `provider` (required) - the name of the provider ("aws" in this case).
    - `tag_key` (required) - the key of the tag to auto-join on.
    - `tag_value` (required) - the value of the tag to auto-join on.
    - `region` (optional) - the AWS region to authenticate in.
	- `addr_type` (optional) - the type of address to discover: `private_v4`, `public_v4`, `public_v6`. Default is `private_v4`. (>= 1.0)
    - `access_key_id` (optional) - the AWS access key for authentication (see below for more information about authenticating).
    - `secret_access_key` (optional) - the AWS secret access key for authentication (see below for more information about authenticating).

    #### Authentication &amp; Precedence

    - Static credentials `access_key_id=... secret_access_key=...`
    - Environment variables (`AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`)
    - Shared credentials file (`~/.aws/credentials` or the path specified by `AWS_SHARED_CREDENTIALS_FILE`)
    - ECS task role metadata (container-specific).
    - EC2 instance role metadata.

    The only required IAM permission is `ec2:DescribeInstances`, and it is
    recommended that you make a dedicated key used only for auto-joining. If the
    region is omitted it will be discovered through the local instance's [EC2
    metadata
    endpoint](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html).

    ### Microsoft Azure

    This returns the first private IP address of all servers in the given region
    which have the given `tag_key` and `tag_value` in the tenant and subscription, or in
    the given `resource_group` of a `vm_scale_set` for Virtual Machine Scale Sets.

    ```sh
    $ consul agent -retry-join "provider=azure tag_name=... tag_value=... tenant_id=... client_id=... subscription_id=... secret_access_key=..."
    ```

    ```json
    {
      "retry_join": ["provider=azure tag_name=... tag_value=... tenant_id=... client_id=... subscription_id=... secret_access_key=..."]
    }
    ```

    - `provider` (required) - the name of the provider ("azure" in this case).
    - `tenant_id` (required) - the tenant to join machines in.
    - `client_id` (required) - the client to authenticate with.
    - `secret_access_key` (required) - the secret client key.

    Use these configuration parameters when using tags:
    - `tag_name` - the name of the tag to auto-join on.
    - `tag_value` - the value of the tag to auto-join on.

    Use these configuration parameters when using Virtual Machine Scale Sets (Consul 1.0.3 and later):
    - `resource_group` - the name of the resource group to filter on.
    - `vm_scale_set` - the name of the virtual machine scale set to filter on.

    When using tags the only permission needed is the `ListAll` method for `NetworkInterfaces`. When using
    Virtual Machine Scale Sets the only role action needed is `Microsoft.Compute/virtualMachineScaleSets/*/read`.

    ### Google Compute Engine

    This returns the first private IP address of all servers in the given
    project which have the given `tag_value`.

    ```sh
    $ consul agent -retry-join "provider=gce project_name=... tag_value=..."
    ```

    ```json
    {
      "retry_join": ["provider=gce project_name=... tag_value=..."]
    }
    ```

    - `provider` (required) - the name of the provider ("gce" in this case).
    - `tag_value` (required) - the value of the tag to auto-join on.
    - `project_name` (optional) - the name of the project to auto-join on. Discovered if not set.
    - `zone_pattern` (optional) - the list of zones can be restricted through an RE2 compatible regular expression. If omitted, servers in all zones are returned.
    - `credentials_file` (optional) - the credentials file for authentication. See below for more information.

    #### Authentication &amp; Precedence

    - Use credentials from `credentials_file`, if provided.
    - Use JSON file from `GOOGLE_APPLICATION_CREDENTIALS` environment variable.
    - Use JSON file in a location known to the gcloud command-line tool.
      - On Windows, this is `%APPDATA%/gcloud/application_default_credentials.json`.
      - On other systems, `$HOME/.config/gcloud/application_default_credentials.json`.
    - On Google Compute Engine, use credentials from the metadata
      server. In this final case any provided scopes are ignored.

    Discovery requires a [GCE Service
    Account](https://cloud.google.com/compute/docs/access/service-accounts).
    Credentials are searched using the following paths, in order of precedence.

    ### IBM SoftLayer

    This returns the first private IP address of all servers for the given
    datacenter with the given `tag_value`.

    ```sh
    $ consul agent -retry-join "provider=softlayer datacenter=... tag_value=... username=... api_key=..."
    ```

    ```json
    {
      "retry_join": ["provider=softlayer datacenter=... tag_value=... username=... api_key=..."]
    }
    ```

    - `provider` (required) - the name of the provider ("softlayer" in this case).
    - `datacenter` (required) - the name of the datacenter to auto-join in.
    - `tag_value` (required) - the value of the tag to auto-join on.
    - `username` (required) - the username to use for auth.
    - `api_key` (required) - the api key to use for auth.

    ### Aliyun (Alibaba Cloud)

    This returns the first private IP address of all servers for the given
    `region` with the given `tag_key` and `tag_value`.

    ```sh
    $ consul agent -retry-join "provider=aliyun region=... tag_key=consul tag_value=... access_key_id=... access_key_secret=..."
    ```

    ```json
    {
      "retry_join": ["provider=aliyun region=... tag_key=consul tag_value=... access_key_id=... access_key_secret=..."]
    }
    ```

    - `provider` (required) - the name of the provider ("aliyun" in this case).
    - `region` (required) - the name of the region.
    - `tag_key` (required) - the key of the tag to auto-join on.
    - `tag_value` (required) - the value of the tag to auto-join on.
    - `access_key_id` (required) -the access key to use for auth.
    - `access_key_secret` (required) - the secret key to use for auth.

	The required RAM permission is `ecs:DescribeInstances`.
	It is recommended you make a dedicated key used only for auto-joining.

    ### Digital Ocean

    This returns the first private IP address of all servers for the given
    `region` with the given `tag_name`.

    ```sh
    $ consul agent -retry-join "provider=digitalocean region=... tag_name=... api_token=..."
    ```

    ```json
    {
      "retry_join": ["provider=digitalocean region=... tag_name=... api_token=..."]
    }
    ```

    - `provider` (required) - the name of the provider ("digitalocean" in this case).
    - `region` (required) - the name of the region.
    - `tag_name` (required) - the value of the tag to auto-join on.
    - `api_token` (required) -the token to use for auth.

    ### Openstack

    This returns the first private IP address of all servers for the given
    `region` with the given `tag_key` and `tag_value`.

    ```sh
    $ consul agent -retry-join "provider=os tag_key=consul tag_value=server username=... password=... auth_url=..."
    ```

    ```json
    {
      "retry_join": ["provider=os tag_key=consul tag_value=server username=... password=... auth_url=..."]
    }
    ```

    - `provider` (required) - the name of the provider ("os" in this case).
    - `tag_key` (required) - the key of the tag to auto-join on.
    - `tag_value` (required) - the value of the tag to auto-join on.
    - `project_id` (optional) - the id of the project (tenant id).
    - `username` (optional) - the username to use for auth.
    - `password` (optional) - the password to use for auth.
    - `token` (optional) - the token to use for auth.
    - `auth_url` (optional) - the identity endpoint to use for auth.
    - `insecure` (optional) - indicates whether the API certificate should not be checked. Any value means `true`.

    The configuration can also be provided by environment variables.

    ### Scaleway

    This returns the first private IP address of all servers for the given
    `region` with the given `tag_key` and `tag_value`.

    ```sh
    $ consul agent -retry-join "provider=scaleway organization=my-org tag_name=consul-server token=... region=..."
    ```

    ```json
    {
      "retry_join": ["provider=scaleway organization=my-org tag_name=consul-server token=... region=..."]
    }
    ```

    - `provider` (required) - the name of the provider ("scaleway" in this case).
    - `region` (required) - the name of the region.
    - `tag_name` (required) - the name of the tag to auto-join on.
    - `organization` (optional) - the organization access key to use for auth.
    - `token` (optional) - the token to use for auth.

* <a name="_retry_interval"></a><a href="#_retry_interval">`-retry-interval`</a> - Time
  to wait between join attempts. Defaults to 30s.

* <a name="_retry_max"></a><a href="#_retry_max">`-retry-max`</a> - The maximum number
  of [`-join`](#_join) attempts to be made before exiting
  with return code 1. By default, this is set to 0 which is interpreted as infinite
  retries.

* <a name="_join_wan"></a><a href="#_join_wan">`-join-wan`</a> - Address of another
  wan agent to join upon starting up. This can be
  specified multiple times to specify multiple WAN agents to join. If Consul is
  unable to join with any of the specified addresses, agent startup will
  fail. By default, the agent won't [`-join-wan`](#_join_wan) any nodes when it starts up.

* <a name="_retry_join_wan"></a><a href="#_retry_join_wan">`-retry-join-wan`</a> - Similar
  to [`retry-join`](#_retry_join) but allows retrying a wan join if the first attempt fails.
  This is useful for cases where we know the address will become available eventually.
  As of Consul 0.9.3 [Cloud Auto-Joining](#cloud-auto-joining) is supported as well.

* <a name="_retry_interval_wan"></a><a href="#_retry_interval_wan">`-retry-interval-wan`</a> - Time
  to wait between [`-join-wan`](#_join_wan) attempts.
  Defaults to 30s.

* <a name="_retry_max_wan"></a><a href="#_retry_max_wan">`-retry-max-wan`</a> - The maximum
  number of [`-join-wan`](#_join_wan) attempts to be made before exiting with return code 1.
  By default, this is set to 0 which is interpreted as infinite retries.

* <a name="_log_level"></a><a href="#_log_level">`-log-level`</a> - The level of logging to
  show after the Consul agent has started. This defaults to "info". The available log levels are
  "trace", "debug", "info", "warn", and "err". You can always connect to an
  agent via [`consul monitor`](/docs/commands/monitor.html) and use any log level. Also, the
  log level can be changed during a config reload.

* <a name="_node"></a><a href="#_node">`-node`</a> - The name of this node in the cluster.
  This must be unique within the cluster. By default this is the hostname of the machine.

* <a name="_node_id"></a><a href="#_node_id">`-node-id`</a> - Available in Consul 0.7.3 and later, this
  is a unique identifier for this node across all time, even if the name of the node or address
  changes. This must be in the form of a hex string, 36 characters long, such as
  `adf4238a-882b-9ddc-4a9d-5b6758e4159e`. If this isn't supplied, which is the most common case, then
  the agent will generate an identifier at startup and persist it in the <a href="#_data_dir">data directory</a>
  so that it will remain the same across agent restarts. Information from the host will be used to
  generate a deterministic node ID if possible, unless [`-disable-host-node-id`](#_disable_host_node_id) is
  set to true.

* <a name="_node_meta"></a><a href="#_node_meta">`-node-meta`</a> - Available in Consul 0.7.3 and later,
  this specifies an arbitrary metadata key/value pair to associate with the node, of the form `key:value`.
  This can be specified multiple times. Node metadata pairs have the following restrictions:
  - A maximum of 64 key/value pairs can be registered per node.
  - Metadata keys must be between 1 and 128 characters (inclusive) in length
  - Metadata keys must contain only alphanumeric, `-`, and `_` characters.
  - Metadata keys must not begin with the `consul-` prefix; that is reserved for internal use by Consul.
  - Metadata values must be between 0 and 512 (inclusive) characters in length.
  - Metadata values for keys begining with `rfc1035-` are encoded verbatim in DNS TXT requests, otherwise
    the metadata kv-pair is encoded according [RFC1464](https://www.ietf.org/rfc/rfc1464.txt).

* <a name="_pid_file"></a><a href="#_pid_file">`-pid-file`</a> - This flag provides the file
  path for the agent to store its PID. This is useful for sending signals (for example, `SIGINT`
  to close the agent or `SIGHUP` to update check definite

* <a name="_protocol"></a><a href="#_protocol">`-protocol`</a> - The Consul protocol version to
  use. This defaults to the latest version. This should be set only when [upgrading](/docs/upgrading.html).
  You can view the protocol versions supported by Consul by running `consul -v`.

* <a name="_raft_protocol"></a><a href="#_raft_protocol">`-raft-protocol`</a> - This controls the internal
  version of the Raft consensus protocol used for server communications. This must be set to 3 in order to
  gain access to Autopilot features, with the exception of [`cleanup_dead_servers`](#cleanup_dead_servers).
  Defaults to 3 in Consul 1.0.0 and later (defaulted to 2 previously). See
  [Raft Protocol Version Compatibility](/docs/upgrade-specific.html#raft-protocol-version-compatibility)
  for more details.

* <a name="_recursor"></a><a href="#_recursor">`-recursor`</a> - Specifies the address of an upstream DNS
  server. This option may be provided multiple times, and is functionally
  equivalent to the [`recursors` configuration option](#recursors).

* <a name="_rejoin"></a><a href="#_rejoin">`-rejoin`</a> - When provided, Consul will ignore a
  previous leave and attempt to rejoin the cluster when starting. By default, Consul treats leave
  as a permanent intent and does not attempt to join the cluster again when starting. This flag
  allows the previous state to be used to rejoin the cluster.

* <a name="_segment"></a><a href="#_segment">`-segment`</a> - (Enterprise-only) This flag is used to set
  the name of the network segment the agent belongs to. An agent can only join and communicate with other agents
  within its network segment. See the [Network Segments Guide](/docs/guides/segments.html) for more details.
  By default, this is an empty string, which is the default network segment.

* <a name="_server"></a><a href="#_server">`-server`</a> - This flag is used to control if an
  agent is in server or client mode. When provided,
  an agent will act as a Consul server. Each Consul cluster must have at least one server and ideally
  no more than 5 per datacenter. All servers participate in the Raft consensus algorithm to ensure that
  transactions occur in a consistent, linearizable manner. Transactions modify cluster state, which
  is maintained on all server nodes to ensure availability in the case of node failure. Server nodes also
  participate in a WAN gossip pool with server nodes in other datacenters. Servers act as gateways
  to other datacenters and forward traffic as appropriate.

* <a name="_non_voting_server"></a><a href="#_non_voting_server">`-non-voting-server`</a> - (Enterprise-only)
  This flag is used to make the server not participate in the Raft quorum, and have it only receive the data
  replication stream. This can be used to add read scalability to a cluster in cases where a high volume of
  reads to servers are needed.

* <a name="_syslog"></a><a href="#_syslog">`-syslog`</a> - This flag enables logging to syslog. This
  is only supported on Linux and OSX. It will result in an error if provided on Windows.

* <a name="_ui"></a><a href="#_ui">`-ui`</a> - Enables the built-in web UI
  server and the required HTTP routes. This eliminates the need to maintain the
  Consul web UI files separately from the binary.

* <a name="_ui_dir"></a><a href="#_ui_dir">`-ui-dir`</a> - This flag provides the directory containing
  the Web UI resources for Consul. This will automatically enable the Web UI. The directory must be
  readable to the agent. Starting with Consul version 0.7.0 and later, the Web UI assets are included in the binary so this flag is no longer necessary; specifying only the `-ui` flag is enough to enable the Web UI. Specifying both the '-ui' and '-ui-dir' flags will result in an error.

## <a name="configuration_files"></a>Configuration Files

In addition to the command-line options, configuration can be put into
files. This may be easier in certain situations, for example when Consul is
being configured using a configuration management system.

The configuration files are JSON formatted, making them easily readable
and editable by both humans and computers. The configuration is formatted
as a single JSON object with configuration within it.

Configuration files are used for more than just setting up the agent,
they are also used to provide check and service definitions. These are used
to announce the availability of system servers to the rest of the cluster.
They are documented separately under [check configuration](/docs/agent/checks.html) and
[service configuration](/docs/agent/services.html) respectively. The service and check
definitions support being updated during a reload.

#### Example Configuration File

```javascript
{
  "datacenter": "east-aws",
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "foobar",
  "server": true,
  "watches": [
    {
        "type": "checks",
        "handler": "/usr/bin/health-check-handler.sh"
    }
  ],
  "telemetry": {
     "statsite_address": "127.0.0.1:2180"
  }
}
```

#### Example Configuration File, with TLS

```javascript
{
  "datacenter": "east-aws",
  "data_dir": "/opt/consul",
  "log_level": "INFO",
  "node_name": "foobar",
  "server": true,
  "addresses": {
    "https": "0.0.0.0"
  },
  "ports": {
    "https": 8080
  },
  "key_file": "/etc/pki/tls/private/my.key",
  "cert_file": "/etc/pki/tls/certs/my.crt",
  "ca_file": "/etc/pki/tls/certs/ca-bundle.crt"
}
```

See, especially, the use of the `ports` setting:

```javascript
"ports": {
  "https": 8080
}
```

Consul will not enable TLS for the HTTP API unless the `https` port has been assigned a port number `> 0`.

#### Configuration Key Reference

* <a name="acl_datacenter"></a><a href="#acl_datacenter">`acl_datacenter`</a> - This designates
  the datacenter which is authoritative for ACL information. It must be provided to enable ACLs.
  All servers and datacenters must agree on the ACL datacenter. Setting it on the servers is all
  you need for cluster-level enforcement, but for the APIs to forward properly from the clients,
  it must be set on them too. In Consul 0.8 and later, this also enables agent-level enforcement
  of ACLs. Please see the [ACL Guide](/docs/guides/acl.html) for more details.

* <a name="acl_default_policy"></a><a href="#acl_default_policy">`acl_default_policy`</a> - Either
  "allow" or "deny"; defaults to "allow". The default policy controls the behavior of a token when
  there is no matching rule. In "allow" mode, ACLs are a blacklist: any operation not specifically
  prohibited is allowed. In "deny" mode, ACLs are a whitelist: any operation not
  specifically allowed is blocked. *Note*: this will not take effect until you've set `acl_datacenter`
  to enable ACL support.

* <a name="acl_down_policy"></a><a href="#acl_down_policy">`acl_down_policy`</a> - Either
  "allow", "deny" or "extend-cache"; "extend-cache" is the default. In the case that the
  policy for a token cannot be read from the [`acl_datacenter`](#acl_datacenter) or leader
  node, the down policy is applied. In "allow" mode, all actions are permitted, "deny" restricts
  all operations, and "extend-cache" allows any cached ACLs to be used, ignoring their TTL
  values. If a non-cached ACL is used, "extend-cache" acts like "deny".

* <a name="acl_agent_master_token"></a><a href="#acl_agent_master_token">`acl_agent_master_token`</a> -
  Used to access <a href="/api/agent.html">agent endpoints</a> that require agent read
  or write privileges, or node read privileges, even if Consul servers aren't present to validate
  any tokens. This should only be used by operators during outages, regular ACL tokens should normally
  be used by applications. This was added in Consul 0.7.2 and is only used when
  <a href="#acl_enforce_version_8">`acl_enforce_version_8`</a> is set to true. Please see
  [ACL Agent Master Token](/docs/guides/acl.html#acl-agent-master-token) for more details.

*   <a name="acl_agent_token"></a><a href="#acl_agent_token">`acl_agent_token`</a> - Used for clients
    and servers to perform internal operations. If this isn't specified, then the
    <a href="#acl_token">`acl_token`</a> will be used. This was added in Consul 0.7.2.

    This token must at least have write access to the node name it will register as in order to set any
    of the node-level information in the catalog such as metadata, or the node's tagged addresses. There
    are other places this token is used, please see [ACL Agent Token](/docs/guides/acl.html#acl-agent-token)
    for more details.

* <a name="acl_enforce_version_8"></a><a href="#acl_enforce_version_8">`acl_enforce_version_8`</a> -
  Used for clients and servers to determine if enforcement should occur for new ACL policies being
  previewed before Consul 0.8. Added in Consul 0.7.2, this defaults to false in versions of
  Consul prior to 0.8, and defaults to true in Consul 0.8 and later. This helps ease the
  transition to the new ACL features by allowing policies to be in place before enforcement begins.
  Please see the [ACL Guide](/docs/guides/acl.html#version_8_acls) for more details.

*   <a name="acl_master_token"></a><a href="#acl_master_token">`acl_master_token`</a> - Only used
    for servers in the [`acl_datacenter`](#acl_datacenter). This token will be created with management-level
    permissions if it does not exist. It allows operators to bootstrap the ACL system
    with a token ID that is well-known.

    The `acl_master_token` is only installed when a server acquires cluster leadership. If
    you would like to install or change the `acl_master_token`, set the new value for `acl_master_token`
    in the configuration for all servers. Once this is done, restart the current leader to force a
    leader election. If the `acl_master_token` is not supplied, then the servers do not create a master
    token. When you provide a value, it can be any string value. Using a UUID would ensure that it looks
    the same as the other tokens, but isn't strictly necessary.

*   <a name="acl_replication_token"></a><a href="#acl_replication_token">`acl_replication_token`</a> -
    Only used for servers outside the [`acl_datacenter`](#acl_datacenter) running Consul 0.7 or later.
    When provided, this will enable [ACL replication](/docs/guides/acl.html#replication) using this
    token to retrieve and replicate the ACLs to the non-authoritative local datacenter. In Consul 0.9.1
    and later you can enable ACL replication using [`enable_acl_replication`](#enable_acl_replication)
    and then set the token later using the [agent token API](/api/agent.html#update-acl-tokens) on each
    server. If the `acl_replication_token` is set in the config, it will automatically set
    [`enable_acl_replication`](#enable_acl_replication) to true for backward compatibility.

    If there's a partition or other outage affecting the authoritative datacenter, and the
    [`acl_down_policy`](/docs/agent/options.html#acl_down_policy) is set to "extend-cache", tokens not
    in the cache can be resolved during the outage using the replicated set of ACLs. Please see the
    [ACL Guide](/docs/guides/acl.html#replication) replication section for more details.

* <a name="acl_token"></a><a href="#acl_token">`acl_token`</a> - When provided, the agent will use this
  token when making requests to the Consul servers. Clients can override this token on a per-request
  basis by providing the "?token" query parameter. When not provided, the empty token, which maps to
  the 'anonymous' ACL policy, is used.

* <a name="acl_ttl"></a><a href="#acl_ttl">`acl_ttl`</a> - Used to control Time-To-Live caching of ACLs.
  By default, this is 30 seconds. This setting has a major performance impact: reducing it will cause
  more frequent refreshes while increasing it reduces the number of refreshes. However, because the caches
  are not actively invalidated, ACL policy may be stale up to the TTL value.

*   <a name="addresses"></a><a href="#addresses">`addresses`</a> - This is a nested object that allows
    setting bind addresses. In Consul 1.0 and later these can be set to a space-separated list of
    addresses to bind to, or a [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template)
    template that can potentially resolve to multiple addresses.

    `http` supports binding to a Unix domain socket. A socket can be
    specified in the form `unix:///path/to/socket`. A new domain socket will be
    created at the given path. If the specified file path already exists, Consul
    will attempt to clear the file and create the domain socket in its place. The
    permissions of the socket file are tunable via the [`unix_sockets` config construct](#unix_sockets).

    When running Consul agent commands against Unix socket interfaces, use the
    `-http-addr` argument to specify the path to the socket. You can also place
    the desired values in the `CONSUL_HTTP_ADDR` environment variable.

    For TCP addresses, the variable values should be an IP address with the port. For
    example: `10.0.0.1:8500` and not `10.0.0.1`. However, ports are set separately in the
    <a href="#ports">`ports`</a> structure when defining them in a configuration file.

    The following keys are valid:
    - `dns` - The DNS server. Defaults to `client_addr`
    - `http` - The HTTP API. Defaults to `client_addr`
    - `https` - The HTTPS API. Defaults to `client_addr`

* <a name="advertise_addr"></a><a href="#advertise_addr">`advertise_addr`</a> Equivalent to
  the [`-advertise` command-line flag](#_advertise).

* <a name="serf_wan"></a><a href="#serf_wan_bind">`serf_wan`</a> Equivalent to
  the [`-serf-wan-bind` command-line flag](#_serf_wan_bind).

* <a name="serf_lan"></a><a href="#serf_lan_bind">`serf_lan`</a> Equivalent to
  the [`-serf-lan-bind` command-line flag](#_serf_lan_bind).

* <a name="advertise_addr_wan"></a><a href="#advertise_addr_wan">`advertise_addr_wan`</a> Equivalent to
  the [`-advertise-wan` command-line flag](#_advertise-wan).

*   <a name="autopilot"></a><a href="#autopilot">`autopilot`</a> Added in Consul 0.8, this object
    allows a number of sub-keys to be set which can configure operator-friendly settings for Consul servers.
    For more information about Autopilot, see the [Autopilot Guide](/docs/guides/autopilot.html).

    The following sub-keys are available:

    * <a name="cleanup_dead_servers"></a><a href="#cleanup_dead_servers">`cleanup_dead_servers`</a> - This controls
      the automatic removal of dead server nodes periodically and whenever a new server is added to the cluster.
      Defaults to `true`.

    * <a name="last_contact_threshold"></a><a href="#last_contact_threshold">`last_contact_threshold`</a> - Controls
      the maximum amount of time a server can go without contact from the leader before being considered unhealthy.
      Must be a duration value such as `10s`. Defaults to `200ms`.

    * <a name="max_trailing_logs"></a><a href="#max_trailing_logs">`max_trailing_logs`</a> - Controls
      the maximum number of log entries that a server can trail the leader by before being considered unhealthy. Defaults
      to 250.

    * <a name="server_stabilization_time"></a><a href="#server_stabilization_time">`server_stabilization_time`</a> -
      Controls the minimum amount of time a server must be stable in the 'healthy' state before being added to the
      cluster. Only takes effect if all servers are running Raft protocol version 3 or higher. Must be a duration value
      such as `30s`. Defaults to `10s`.

    * <a name="redundancy_zone_tag"></a><a href="#redundancy_zone_tag">`redundancy_zone_tag`</a> - (Enterprise-only)
      This controls the [`-node-meta`](#_node_meta) key to use when Autopilot is separating servers into zones for
      redundancy. Only one server in each zone can be a voting member at one time. If left blank (the default), this
      feature will be disabled.

    * <a name="disable_upgrade_migration"></a><a href="#disable_upgrade_migration">`disable_upgrade_migration`</a> - (Enterprise-only)
      If set to `true`, this setting will disable Autopilot's upgrade migration strategy in Consul Enterprise of waiting
      until enough newer-versioned servers have been added to the cluster before promoting any of them to voters. Defaults
      to `false`.

* <a name="bootstrap"></a><a href="#bootstrap">`bootstrap`</a> Equivalent to the
  [`-bootstrap` command-line flag](#_bootstrap).

* <a name="bootstrap_expect"></a><a href="#bootstrap_expect">`bootstrap_expect`</a> Equivalent
  to the [`-bootstrap-expect` command-line flag](#_bootstrap_expect).

* <a name="bind_addr"></a><a href="#bind_addr">`bind_addr`</a> Equivalent to the
  [`-bind` command-line flag](#_bind).

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

* <a name="client_addr"></a><a href="#client_addr">`client_addr`</a> Equivalent to the
  [`-client` command-line flag](#_client).

* <a name="datacenter"></a><a href="#datacenter">`datacenter`</a> Equivalent to the
  [`-datacenter` command-line flag](#_datacenter).

* <a name="data_dir"></a><a href="#data_dir">`data_dir`</a> Equivalent to the
  [`-data-dir` command-line flag](#_data_dir).

* <a name="disable_anonymous_signature"></a><a href="#disable_anonymous_signature">
  `disable_anonymous_signature`</a> Disables providing an anonymous signature for de-duplication
  with the update check. See [`disable_update_check`](#disable_update_check).

* <a name="disable_host_node_id"></a><a href="#disable_host_node_id">`disable_host_node_id`</a>
  Equivalent to the [`-disable-host-node-id` command-line flag](#_disable_host_node_id).

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

*   <a name="dns_config"></a><a href="#dns_config">`dns_config`</a> This object allows a number
    of sub-keys to be set which can tune how DNS queries are serviced. See this guide on
    [DNS caching](/docs/guides/dns-cache.html) for more detail.

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
      response. When answering a question, Consul will use the complete list of
      matching hosts, shuffle the list randomly, and then limit the number of
      answers to `udp_answer_limit` (default `3`). In environments where
      [RFC 3484 Section 6](https://tools.ietf.org/html/rfc3484#section-6) Rule 9
      is implemented and enforced (i.e. DNS answers are always sorted and
      therefore never random), clients may need to set this value to `1` to
      preserve the expected randomized distribution behavior (note:
      [RFC 3484](https://tools.ietf.org/html/rfc3484) has been obsoleted by
      [RFC 6724](https://tools.ietf.org/html/rfc6724) and as a result it should
      be increasingly uncommon to need to change this value with modern
      resolvers).

* <a name="domain"></a><a href="#domain">`domain`</a> Equivalent to the
  [`-domain` command-line flag](#_domain).

* <a name="enable_acl_replication"></a><a href="#enable_acl_replication">`enable_acl_replication`</a> When
  set on a Consul server, enables [ACL replication](/docs/guides/acl.html#replication) without having to set
  the replication token via [`acl_replication_token`](#acl_replication_token). Instead, enable ACL replication
  and then introduce the token using the [agent token API](/api/agent.html#update-acl-tokens) on each server.
  See [`acl_replication_token`](#acl_replication_token) for more details.

* <a name="enable_agent_tls_for_checks"></a><a href="#enable_agent_tls_for_checks">`enable_agent_tls_for_checks`</a>
  When set, uses a subset of the agent's TLS configuration (`key_file`, `cert_file`, `ca_file`, `ca_path`, and
  `server_name`) to set up the HTTP client for HTTP health checks. This allows services requiring 2-way TLS to
  be checked using the agent's credentials. This was added in Consul 1.0.1 and defaults to false.

* <a name="enable_debug"></a><a href="#enable_debug">`enable_debug`</a> When set, enables some
  additional debugging features. Currently, this is only used to set the runtime profiling HTTP endpoints.

* <a name="enable_script_checks"></a><a href="#enable_script_checks">`enable_script_checks`</a> Equivalent to the
  [`-enable-script-checks` command-line flag](#_enable_script_checks).

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
      access control, Consul's [ACL system](/docs/guides/acl.html) should be used, but this option
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

* <a name="leave_on_terminate"></a><a href="#leave_on_terminate">`leave_on_terminate`</a> If
  enabled, when the agent receives a TERM signal, it will send a `Leave` message to the rest
  of the cluster and gracefully leave. The default behavior for this feature varies based on
  whether or not the agent is running as a client or a server (prior to Consul 0.7 the default
  value was unconditionally set to `false`). On agents in client-mode, this defaults to `true`
  and for agents in server-mode, this defaults to `false`.

* <a name="limits"></a><a href="#limits">`limits`</a> Available in Consul 0.9.3 and later, this
  is a nested object that configures limits that are enforced by the agent. Currently, this only
  applies to agents in client mode, not Consul servers. The following parameters are available:

    *   <a name="rpc_rate"></a><a href="#rpc_rate">`rpc_rate`</a> - Configures the RPC rate
        limiter by setting the maximum request rate that this agent is allowed to make for RPC
        requests to Consul servers, in requests per second. Defaults to infinite, which disables
        rate limiting.
    *   <a name="rpc_rate"></a><a href="#rpc_max_burst">`rpc_max_burst`</a> - The size of the token
        bucket used to recharge the RPC rate limiter. Defaults to 1000 tokens, and each token is
        good for a single RPC call to a Consul server. See https://en.wikipedia.org/wiki/Token_bucket
        for more details about how token bucket rate limiters operate.

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

*   <a name="performance"></a><a href="#performance">`performance`</a> Available in Consul 0.7 and
    later, this is a nested object that allows tuning the performance of different subsystems in
    Consul. See the [Server Performance](/docs/guides/performance.html) guide for more details. The
    following parameters are available:

    *   <a name="leave_drain_time"></a><a href="#leave_drain_time">`leave_drain_time`</a> - A duration
        that a server will dwell during a graceful leave in order to allow requests to be retried against
        other Consul servers. Under normal circumstances, this can prevent clients from experiencing
        "no leader" errors when performing a rolling update of the Consul servers. This was added in
        Consul 1.0. Must be a duration value such as 10s. Defaults to 5s.

    *   <a name="raft_multiplier"></a><a href="#raft_multiplier">`raft_multiplier`</a> - An integer
        multiplier used by Consul servers to scale key Raft timing parameters. Omitting this value
        or setting it to 0 uses default timing described below. Lower values are used to tighten
        timing and increase sensitivity while higher values relax timings and reduce sensitivity.
        Tuning this affects the time it takes Consul to detect leader failures and to perform
        leader elections, at the expense of requiring more network and CPU resources for better
        performance.

        By default, Consul will use a lower-performance timing that's suitable
        for [minimal Consul servers](/docs/guides/performance.html#minumum), currently equivalent
        to setting this to a value of 5 (this default may be changed in future versions of Consul,
        depending if the target minimum server profile changes). Setting this to a value of 1 will
        configure Raft to its highest-performance mode, equivalent to the default timing of Consul
        prior to 0.7, and is recommended for [production Consul servers](/docs/guides/performance.html#production).
        See the note on [last contact](/docs/guides/performance.html#last-contact) timing for more
        details on tuning this parameter. The maximum allowed value is 10.

    *   <a name="rpc_hold_timeout"></a><a href="#rpc_hold_timeout">`rpc_hold_timeout`</a> - A duration
        that a client or server will retry internal RPC requests during leader elections. Under normal
        circumstances, this can prevent clients from experiencing "no leader" errors. This was added in
        Consul 1.0. Must be a duration value such as 10s. Defaults to 7s.

* <a name="ports"></a><a href="#ports">`ports`</a> This is a nested object that allows setting
  the bind ports for the following keys:
    * <a name="dns_port"></a><a href="#dns_port">`dns`</a> - The DNS server, -1 to disable. Default 8600.
    * <a name="http_port"></a><a href="#http_port">`http`</a> - The HTTP API, -1 to disable. Default 8500.
    * <a name="https_port"></a><a href="#https_port">`https`</a> - The HTTPS API, -1 to disable. Default -1 (disabled).
    * <a name="serf_lan_port"></a><a href="#serf_lan_port">`serf_lan`</a> - The Serf LAN port. Default 8301.
    * <a name="serf_wan_port"></a><a href="#serf_wan_port">`serf_wan`</a> - The Serf WAN port. Default 8302.
    * <a name="server_rpc_port"></a><a href="#server_rpc_port">`server`</a> - Server RPC address. Default 8300.

* <a name="protocol"></a><a href="#protocol">`protocol`</a> Equivalent to the
  [`-protocol` command-line flag](#_protocol).

* <a name="raft_protocol"></a><a href="#raft_protocol">`raft_protocol`</a> Equivalent to the
  [`-raft-protocol` command-line flag](#_raft_protocol).

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

* <a name="reconnect_timeout_wan"></a><a href="#reconnect_timeout_wan">`reconnect_timeout_wan`</a> This
  is the WAN equivalent of the <a href="#reconnect_timeout">`reconnect_timeout`</a> parameter, which
  controls how long it takes for a failed server to be completely removed from the WAN pool. This also
  defaults to 72 hours, and must be >= 8 hours.

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

* <a name="retry_join_wan"></a><a href="#retry_join_wan">`retry_join_wan`</a> Equivalent to the
  [`-retry-join-wan` command-line flag](#_retry_join_wan). Takes a list
  of addresses to attempt joining to WAN every [`retry_interval_wan`](#_retry_interval_wan) until at least one
  join works.

* <a name="retry_interval_wan"></a><a href="#retry_interval_wan">`retry_interval_wan`</a> Equivalent to the
  [`-retry-interval-wan` command-line flag](#_retry_interval_wan).

* <a name="segment"></a><a href="#segment">`segment`</a> (Enterprise-only) Equivalent to the
  [`-segment` command-line flag](#_segment).

* <a name="segments"></a><a href="#segments">`segments`</a> (Enterprise-only) This is a list of nested objects that allows setting
  the bind/advertise information for network segments. This can only be set on servers. See the
  [Network Segments Guide](/docs/guides/segments.html) for more details.
    * <a name="segment_name"></a><a href="#segment_name">`name`</a> - The name of the segment. Must be a string between
    1 and 64 characters in length.
    * <a name="segment_bind"></a><a href="#segment_bind">`bind`</a> - The bind address to use for the segment's gossip layer.
    Defaults to the [`-bind`](#_bind) value if not provided.
    * <a name="segment_port"></a><a href="#segment_port">`port`</a> - The port to use for the segment's gossip layer (required).
    * <a name="segment_advertise"></a><a href="#segment_advertise">`advertise`</a> - The advertise address to use for the
    segment's gossip layer. Defaults to the [`-advertise`](#_advertise) value if not provided.
    * <a name="segment_rpc_listener"></a><a href="#segment_rpc_listener">`rpc_listener`</a> - If true, a separate RPC listener will
    be started on this segment's [`-bind`](#_bind) address on the rpc port. Only valid if the segment's bind address differs from the
    [`-bind`](#_bind) address. Defaults to false.

* <a name="server"></a><a href="#server">`server`</a> Equivalent to the
  [`-server` command-line flag](#_server).

* <a name="non_voting_server"></a><a href="#non_voting_server">`non_voting_server`</a> - Equivalent to the
  [`-non-voting-server` command-line flag](#_non_voting_server).

* <a name="server_name"></a><a href="#server_name">`server_name`</a> When provided, this overrides
  the [`node_name`](#_node) for the TLS certificate. It can be used to ensure that the certificate
  name matches the hostname we declare.

* <a name="session_ttl_min"></a><a href="#session_ttl_min">`session_ttl_min`</a>
  The minimum allowed session TTL. This ensures sessions are not created with
  TTL's shorter than the specified limit. It is recommended to keep this limit
  at or above the default to encourage clients to send infrequent heartbeats.
  Defaults to 10s.

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

* <a name="start_join"></a><a href="#start_join">`start_join`</a> An array of strings specifying addresses
  of nodes to [`-join`](#_join) upon startup. Note that using
  <a href="#retry_join">`retry_join`</a> could be more appropriate to help
  mitigate node startup race conditions when automating a Consul cluster
  deployment.

* <a name="start_join_wan"></a><a href="#start_join_wan">`start_join_wan`</a> An array of strings specifying
  addresses of WAN nodes to [`-join-wan`](#_join_wan) upon startup.

*   <a name="telemetry"></a><a href="#telemetry">`telemetry`</a> This is a nested object that configures where Consul
    sends its runtime telemetry, and contains the following keys:

    * <a name="telemetry-circonus_api_token"></a><a href="#telemetry-circonus_api_token">`circonus_api_token`</a>
      A valid API Token used to create/manage check. If provided, metric management is enabled.

    * <a name="telemetry-circonus_api_app"></a><a href="#telemetry-circonus_api_app">`circonus_api_app`</a>
      A valid app name associated with the API token. By default, this is set to "consul".

    * <a name="telemetry-circonus_api_url"></a><a href="#telemetry-circonus_api_url">`circonus_api_url`</a>
      The base URL to use for contacting the Circonus API. By default, this is set to "https://api.circonus.com/v2".

    * <a name="telemetry-circonus_submission_interval"></a><a href="#telemetry-circonus_submission_interval">`circonus_submission_interval`</a>
      The interval at which metrics are submitted to Circonus. By default, this is set to "10s" (ten seconds).

    * <a name="telemetry-circonus_submission_url"></a><a href="#telemetry-circonus_submission_url">`circonus_submission_url`</a>
      The `check.config.submission_url` field, of a Check API object, from a previously created HTTPTRAP check.

    * <a name="telemetry-circonus_check_id"></a><a href="#telemetry-circonus_check_id">`circonus_check_id`</a>
      The Check ID (not **check bundle**) from a previously created HTTPTRAP check. The numeric portion of the `check._cid` field in the Check API object.

    * <a name="telemetry-circonus_check_force_metric_activation"></a><a href="#telemetry-circonus_check_force_metric_activation">`circonus_check_force_metric_activation`</a>
      Force activation of metrics which already exist and are not currently active. If check management is enabled, the default behavior is to add new metrics as they are encoutered. If the metric already exists in the check, it will **not** be activated. This setting overrides that behavior. By default, this is set to false.

    * <a name="telemetry-circonus_check_instance_id"></a><a href="#telemetry-circonus_check_instance_id">`circonus_check_instance_id`</a>
      Uniquely identifies the metrics coming from this *instance*. It can be used to maintain metric continuity with transient or ephemeral instances as they move around within an infrastructure. By default, this is set to hostname:application name (e.g. "host123:consul").

    * <a name="telemetry-circonus_check_search_tag"></a><a href="#telemetry-circonus_check_search_tag">`circonus_check_search_tag`</a>
      A special tag which, when coupled with the instance id, helps to narrow down the search results when neither a Submission URL or Check ID is provided. By default, this is set to service:application name (e.g. "service:consul").

    * <a name="telemetry-circonus_check_display_name"</a><a href="#telemetry-circonus_check_display_name">`circonus_check_display_name`</a>
      Specifies a name to give a check when it is created. This name is displayed in the Circonus UI Checks list. Available in Consul 0.7.2 and later.

    * <a name="telemetry-circonus_check_tags"</a><a href="#telemetry-circonus_check_tags">`circonus_check_tags`</a>
      Comma separated list of additional tags to add to a check when it is created. Available in Consul 0.7.2 and later.

    * <a name="telemetry-circonus_broker_id"></a><a href="#telemetry-circonus_broker_id">`circonus_broker_id`</a>
      The ID of a specific Circonus Broker to use when creating a new check. The numeric portion of `broker._cid` field in a Broker API object. If metric management is enabled and neither a Submission URL nor Check ID is provided, an attempt will be made to search for an existing check using Instance ID and Search Tag. If one is not found, a new HTTPTRAP check will be created. By default, this is not used and a random Enterprise Broker is selected, or the default Circonus Public Broker.

    * <a name="telemetry-circonus_broker_select_tag"></a><a href="#telemetry-circonus_broker_select_tag">`circonus_broker_select_tag`</a>
      A special tag which will be used to select a Circonus Broker when a Broker ID is not provided. The best use of this is to as a hint for which broker should be used based on *where* this particular instance is running (e.g. a specific geo location or datacenter, dc:sfo). By default, this is left blank and not used.

    * <a name="telemetry-disable_hostname"></a><a href="#telemetry-disable_hostname">`disable_hostname`</a>
      This controls whether or not to prepend runtime telemetry with the machine's hostname, defaults to false.

    * <a name="telemetry-dogstatsd_addr"></a><a href="#telemetry-dogstatsd_addr">`dogstatsd_addr`</a> This provides the
      address of a DogStatsD instance in the format `host:port`. DogStatsD is a protocol-compatible flavor of
      statsd, with the added ability to decorate metrics with tags and event information. If provided, Consul will
      send various telemetry information to that instance for aggregation. This can be used to capture runtime
      information.

    * <a name="telemetry-dogstatsd_tags"></a><a href="#telemetry-dogstatsd_tags">`dogstatsd_tags`</a> This provides a list of global tags
      that will be added to all telemetry packets sent to DogStatsD. It is a list of strings, where each string
      looks like "my_tag_name:my_tag_value".

    * <a name="telemetry-filter_default"></a><a href="#telemetry-filter_default">`filter_default`</a>
     This controls whether to allow metrics that have not been specified by the filter. Defaults to `true`, which will
     allow all metrics when no filters are provided. When set to `false` with no filters, no metrics will be sent.

    * <a name="telemetry-metrics_prefix"></a><a href="#telemetry-metrics_prefix">`metrics_prefix`</a>
      The prefix used while writing all telemetry data. By default, this is set to "consul". This was added
      in Consul 1.0. For previous versions of Consul, use the config option `statsite_prefix` in this
      same structure. This was renamed in Consul 1.0 since this prefix applied to all telemetry providers,
      not just statsite.

    * <a name="telemetry-prefix_filter"></a><a href="#telemetry-prefix_filter">`prefix_filter`</a>
      This is a list of filter rules to apply for allowing/blocking metrics by prefix in the following format:

        ```javascript
        [
          "+consul.raft.apply",
          "-consul.http",
          "+consul.http.GET"
        ]
        ```
      A leading "<b>+</b>" will enable any metrics with the given prefix, and a leading "<b>-</b>" will block them. If there
      is overlap between two rules, the more specific rule will take precedence. Blocking will take priority if the same
      prefix is listed multiple times.

    * <a name="telemetry-enable_deprecated_names"></a><a href="#telemetry-enable_deprecated_names">`enable_deprecated_names`
      </a>Added in Consul 1.0, this enables old metric names of the format `consul.consul...` to be sent alongside
      other metrics. Defaults to false.

    * <a name="telemetry-statsd_address"></a><a href="#telemetry-statsd_address">`statsd_address`</a> This provides the
      address of a statsd instance in the format `host:port`. If provided, Consul will send various telemetry information to that instance for
      aggregation. This can be used to capture runtime information. This sends UDP packets only and can be used with
      statsd or statsite.

    * <a name="telemetry-statsite_address"></a><a href="#telemetry-statsite_address">`statsite_address`</a> This provides
      the address of a statsite instance in the format `host:port`. If provided, Consul will stream various telemetry information to that instance
      for aggregation. This can be used to capture runtime information. This streams via TCP and can only be used with
      statsite.

* <a name="syslog_facility"></a><a href="#syslog_facility">`syslog_facility`</a> When
  [`enable_syslog`](#enable_syslog) is provided, this controls to which
  facility messages are sent. By default, `LOCAL0` will be used.

* <a name="tls_min_version"></a><a href="#tls_min_version">`tls_min_version`</a> Added in Consul
  0.7.4, this specifies the minimum supported version of TLS. Accepted values are "tls10", "tls11"
  or "tls12". This defaults to "tls10". WARNING: TLS 1.1 and lower are generally considered less
  secure; avoid using these if possible. This will be changed to default to "tls12" in Consul 0.8.0.

* <a name="tls_cipher_suites"></a><a href="#tls_cipher_suites">`tls_cipher_suites`</a> Added in Consul
  0.8.2, this specifies the list of supported ciphersuites as a comma-separated-list. The list of all
  available ciphersuites is available in the [Golang TLS documentation](https://golang.org/src/crypto/tls/cipher_suites.go).

* <a name="tls_prefer_server_cipher_suites"></a><a href="#tls_prefer_server_cipher_suites">
  `tls_prefer_server_cipher_suites`</a> Added in Consul 0.8.2, this will cause Consul to prefer the
  server's ciphersuite over the client ciphersuites.

*   <a name="translate_wan_addrs"</a><a href="#translate_wan_addrs">`translate_wan_addrs`</a> If
    set to true, Consul will prefer a node's configured <a href="#_advertise-wan">WAN address</a>
    when servicing DNS and HTTP requests for a node in a remote datacenter. This allows the node to
    be reached within its own datacenter using its local address, and reached from other datacenters
    using its WAN address, which is useful in hybrid setups with mixed networks. This is disabled by
    default.

    Starting in Consul 0.7 and later, node addresses in responses to HTTP requests will also prefer a
    node's configured <a href="#_advertise-wan">WAN address</a> when querying for a node in a remote
    datacenter. An [`X-Consul-Translate-Addresses`](/api/index.html#translated-addresses) header
    will be present on all responses when translation is enabled to help clients know that the addresses
    may be translated. The `TaggedAddresses` field in responses also have a `lan` address for clients that
    need knowledge of that address, regardless of translation.

    The following endpoints translate addresses:
    - [`/v1/catalog/nodes`](/api/catalog.html#catalog_nodes)
    - [`/v1/catalog/node/<node>`](/api/catalog.html#catalog_node)
    - [`/v1/catalog/service/<service>`](/api/catalog.html#catalog_service)
    - [`/v1/health/service/<service>`](/api/health.html#health_service)
    - [`/v1/query/<query or name>/execute`](/api/query.html#execute)

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
  true, Consul requires that all outgoing connections
  make use of TLS and that the server provides a certificate that is signed by
  a Certificate Authority from the [`ca_file`](#ca_file) or [`ca_path`](#ca_path). By default,
  this is false, and Consul will not make use of TLS for outgoing connections. This applies to clients
  and servers as both will make outgoing connections.

* <a name="verify_server_hostname"></a><a href="#verify_server_hostname">`verify_server_hostname`</a> - If set to
  true, Consul verifies for all outgoing connections that the TLS certificate presented by the servers
  matches "server.&lt;datacenter&gt;.&lt;domain&gt;" hostname. This implies `verify_outgoing`.
  By default, this is false, and Consul does not verify the hostname of the certificate, only
  that it is signed by a trusted CA. This setting is important to prevent a compromised
  client from being restarted as a server, and thus being able to perform a MITM attack
  or to be added as a Raft peer. This is new in 0.5.1.

* <a name="watches"></a><a href="#watches">`watches`</a> - Watches is a list of watch
  specifications which allow an external process to be automatically invoked when a
  particular data view is updated. See the
   [watch documentation](/docs/agent/watches.html) for more detail. Watches can be
   modified when the configuration is reloaded.

## <a id="ports"></a>Ports Used

Consul requires up to 6 different ports to work properly, some on
TCP, UDP, or both protocols. Below we document the requirements for each
port.

* Server RPC (Default 8300). This is used by servers to handle incoming
  requests from other agents. TCP only.

* Serf LAN (Default 8301). This is used to handle gossip in the LAN.
  Required by all agents. TCP and UDP.

* Serf WAN (Default 8302). This is used by servers to gossip over the
  WAN to other servers. TCP and UDP. As of Consul 0.8, it is recommended to
  enable connection between servers through port 8302 for both TCP and UDP on
  the LAN interface as well for the WAN Join Flooding feature. See also:
  [Consul 0.8.0 CHANGELOG](https://github.com/hashicorp/consul/blob/master/CHANGELOG.md#080-april-5-2017) and [GH-3058](https://github.com/hashicorp/consul/issues/3058)

* HTTP API (Default 8500). This is used by clients to talk to the HTTP
  API. TCP only.

* DNS Interface (Default 8600). Used to resolve DNS queries. TCP and UDP.

## <a id="reloadable-configuration"></a>Reloadable Configuration

Reloading configuration does not reload all configuration items. The
items which are reloaded include:

* Log level
* Checks
* Services
* Watches
* HTTP Client Address
* <a href="#node_meta">Node Metadata</a>
* <a href="#telemetry-prefix_filter">Metric Prefix Filter</a>
* <a href="#discard_check_output">Discard Check Output</a>
