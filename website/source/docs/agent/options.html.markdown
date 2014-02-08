---
layout: "docs"
page_title: "Configuration"
sidebar_current: "docs-agent-config"
---

# Configuration

The agent has various configuration options that can be specified via
the command-line or via configuration files. All of the configuration
options are completely optional and their defaults will be specified
with their descriptions.

When loading configuration, Serf loads the configuration from files
and directories in the order specified. Configuration specified later
will be merged into configuration specified earlier. In most cases,
"merge" means that the later version will override the earlier. But in
some cases, such as event handlers, merging just appends the handlers.
The exact merging behavior will be specified.

Serf also supports reloading of configuration when it receives the
SIGHUP signal. Not all changes are respected, but those that are
are documented below.

## Command-line Options

The options below are all specified on the command-line.

* `-bind` - The address that Serf will bind to for communication with
  other Serf nodes. By default this is "0.0.0.0:7946". Serf nodes may
  have different ports. If a join is specified without a port, we default
  to locally configured port. Serf uses both TCP and UDP and use the
  same port for both, so if you have any firewalls be sure to allow both protocols.
  If this configuration value is changed and no port is specified, the default of
  "7946" will be used. An important compatibility note, protocol version 2
  introduces support for non-consistent ports across the cluster. For more information,
  see the [compatibility page](/docs/compatibility.html).

* `-advertise` - The advertise flag is used to change the address that we
  advertise to other nodes in the cluster. By default, the bind address is
  advertised. However, in some cases (specifically NAT traversal), there may
  be a routable address that cannot be bound to. This flag enables gossiping
  a different address to support this. If this address is not routable, the node
  will be in a constant flapping state, as other nodes will treat the non-routability
  as a failure.

* `-config-file` - A configuration file to load. For more information on
  the format of this file, read the "Configuration Files" section below.
  This option can be specified multiple times to load multiple configuration
  files. If it is specified multiple times, configuration files loaded later
  will merge with configuration files loaded earlier, with the later values
  overriding the earlier values.

* `-config-dir` - A directory of configuration files to load. Serf will
  load all files in this directory ending in ".json" as configuration files
  in alphabetical order. For more information on the format of the configuration
  files, see the "Configuration Files" section below.

* `-discover` - Discover provides a cluster name, which is used with mDNS to
  automatically discover Serf peers. When provided, Serf will respond to mDNS
  queries and periodically poll for new peers. This feature requires a network
  environment that supports multicasting.

* `-encrypt` - Specifies the secret key to use for encryption of Serf
  network traffic. This key must be 16-bytes that are base64 encoded. The
  easiest way to create an encryption key is to use `serf keygen`. All
  nodes within a cluster must share the same encryption key to communicate.

* `-event-handler` - Adds an event handler that Serf will invoke for
  events. This flag can be specified multiple times to define multiple
  event handlers. By default no event handlers are registered. See the
  [event handler page](/docs/agent/event-handlers.html) for more details on
  event handlers as well as a syntax for filtering event handlers by event.
  Event handlers can be changed by reloading the configuration.

* `-join` - Address of another agent to join upon starting up. This can be
  specified multiple times to specify multiple agents to join. If Serf is
  unable to join with any of the specified addresses, agent startup will
  fail. By default, the agent won't join any nodes when it starts up.

* `-replay` - If set, old user events from the past will be replayed for the
  agent/cluster that is joining based on a `-join` configuration. Otherwise,
  past events will be ignored. This configures for the initial join
  only.

* `-log-level` - The level of logging to show after the Serf agent has
  started. This defaults to "info". The available log levels are "trace",
  "debug", "info", "warn", "err". This is the log level that will be shown
  for the agent output, but note you can always connect via `serf monitor`
  to an agent at any log level. The log level can be changed during a
  config reload.

* `-node` - The name of this node in the cluster. This must be unique within
  the cluster. By default this is the hostname of the machine.

* `-profile` - Serf by default is configured to run in a LAN or Local Area
  Network. However, there are cases in which a user may want to use Serf over
  the Internet or (WAN), or even just locally. To support setting the correct
  configuration values for each environment, you can select a timing profile.
  The current choices are "lan", "wan", and "local". This defaults to "local".
  If a "lan" or "local" profile is used over the Internet, or a "local" profile
  over the LAN, a high rate of false failures is risked, as the timing constrains
  are too tight.

* `-protocol` - The Serf protocol version to use. This defaults to the latest
  version. This should be set only when [upgrading](/docs/upgrading.html).
  You can view the protocol versions supported by Serf by running `serf -v`.

* `-role` - **Deprecated** The role of this node, if any. By default this is blank or empty.
  The role can be used by events in order to differentiate members of a
  cluster that may have different functional roles. For example, if you're
  using Serf in a load balancer and web server setup, you only want to add
  web servers to the load balancers, so the role of web servers may be "web"
  and the event handlers can filter on that. This has been deprecated as of
  version 0.4. Instead "-tag role=foo" should be used. The role can be changed
  during a config reload

* `-rpc-addr` - The address that Serf will bind to for the agent's  RPC server.
  By default this is "127.0.0.1:7373", allowing only loopback connections.
  The RPC address is used by other Serf commands, such as  `serf members`,
  in order to query a running Serf agent. It is also used by other applications
  to control Serf using it's [RPC protocol](/docs/agent/rpc.html).

* `-snapshot` - The snapshot flag provides a file path that is used to store
  recovery information, so when Serf restarts it is able to automatically
  re-join the cluster, and avoid replay of events it has already seen. The path
  must be read/writable by Serf, and the directory must allow Serf to create
  other files, so that it can periodically compact the snapshot file.

* `-tag` - The tag flag is used to associate a new key/value pair with the
  agent. The tags are gossiped and can be used to provide additional information
  such as roles, ports, and configuration values to other nodes. Multiple tags
  can be specified per agent. There is a byte size limit for the maximum number
  of tags, but in practice dozens of tags may be used. Tags can be changed during
  a config reload.

## Configuration Files

In addition to the command-line options, configuration can be put into
files. This may be easier in certain situations, for example when Serf is
being configured using a configuration management system.

The configuration files are JSON formatted, making them easily readable
and editable by both humans and computers. The configuration is formatted
at a single JSON object with configuration within it.

#### Example Configuration File

<pre class="prettyprint lang-json">
{
  "tags": {
        "role": "load-balancer",
        "datacenter": "east"
  },

  "event_handlers": [
    "handle.sh",
    "user:deploy=deploy.sh"
  ]
}
</pre>

#### Configuration Key Reference

* `node_name` - Equivalent to the `-node` command-line flag.

* `role` - **Deprecated**. Equivalent to the `-role` command-line flag.

* `tags` - This is a dictionary of tag values. It is the same as specifying
  the `tag` command-line flag once per tag.

* `bind` - Equivalent to the `-bind` command-line flag.

* `advertise` - Equivalent to the `-advertise` command-line flag.

* `discover` - Equivalent to the `-discover` command-line flag.

* `encrypt_key` - Equivalent to the `-encrypt` command-line flag.

* `log_level` - Equivalent to the `-log-level` command-line flag.

* `profile` - Equivalent to the `-profile` command-line flag.

* `protocol` - Equivalent to the `-protocol` command-line flag.

* `rpc_addr` - Equivalent to the `-rpc-addr` command-line flag.

* `event_handlers` - An array of strings specifying the event handlers.
  The format of the strings is equivalent to the format specified for
  the `-event-handler` command-line flag.

* `start_join` - An array of strings specifying addresses of nodes to
  join upon startup.

* `replay_on_join` - Equivalent to the `-replay` command-line flag.

* `snapshot_path` - Equivalent to the `-snapshot` command-line flag.

* `leave_on_terminate` - If enabled, when the agent receives a TERM signal,
  it will send a Leave message to the rest of the cluster and gracefully
  leave. Defaults to false.

* `skip_leave_on_interrupt` - This is the similar to`leave_on_terminate` but
  only affects interrupt handling. By default, an interrupt causes Serf to
  gracefully leave, but setting this to true disables that. Defaults to false.
  Interrupts are usually from a Control-C from a shell. (This was previously
  `leave_on_interrupt` but has since changed).

