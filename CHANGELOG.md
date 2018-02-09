## 1.0.6 (February 9, 2018) ##

BUG FIXES:

* agent: Fixed a panic when using the Azure provider for retry-join. [[GH-3875](https://github.com/hashicorp/consul/issues/3875)]
* agent: Fixed a panic when querying Consul's DNS interface over TCP. [[GH-3877](https://github.com/hashicorp/consul/issues/3877)]

## 1.0.5 (February 7, 2018)

NOTE ON SKIPPED RELEASE 1.0.4:

We found [[GH-3867](https://github.com/hashicorp/consul/issues/3867)] after cutting the 1.0.4 release and pushing the 1.0.4 release tag, so we decided to scuttle that release and push 1.0.5 instead with a fix for that issue.

SECURITY:

* dns: Updated DNS vendor library to pick up bug fix in the DNS server where an open idle connection blocks the accept loop. [[GH-3859](https://github.com/hashicorp/consul/issues/3859)]

FEATURES:

* agent: Added support for gRPC health checks that probe the standard gRPC health endpoint. [[GH-3073](https://github.com/hashicorp/consul/issues/3073)]

IMPROVEMENTS:

* agent: (Consul Enterprise) The `disable_update_check` option to disable Checkpoint now defaults to `true` (this is only in the Enterprise version).
* build: Bumped Go version to 1.9.3. [[GH-3837](https://github.com/hashicorp/consul/issues/3837)]

BUG FIXES:

* agent: (Consul Enterprise) Fixed an issue where the snapshot agent's HTTP client config was being ignored in favor of the HTTP command-line flags.
* agent: Fixed an issue where health checks added to services with tags would cause extra periodic writes to the Consul servers, even if nothing had changed. This could cause extra churn on downstream applications like consul-template or Fabio. [[GH-3845](https://github.com/hashicorp/consul/issues/3845)]
* agent: Fixed several areas where reading from catalog, health, or agent HTTP endpoints could make unintended mofidications to Consul's state in a way that would cause unnecessary anti-entropy syncs back to the Consul servers. This could cause extra churn on downstream applications like consul-template or Fabio. [[GH-3867](https://github.com/hashicorp/consul/issues/3867)]
* agent: Fixed an issue where Serf events for failed Consul servers weren't being proactively processed by the RPC router. This would prvent Consul from proactively choosing a new server, and would instead wait for a failed RPC request before choosing a new server. This exposed clients to a failed request, when often the proactive switching would avoid that. [[GH-3864](https://github.com/hashicorp/consul/issues/3864)]

## 1.0.3 (January 24, 2018)

SECURITY:

* ui: Patched handlebars JS to escape `=` to prevent potential XSS issues. [[GH-3733](https://github.com/hashicorp/consul/issues/3733)]

BREAKING CHANGES:

* agent: Updated Consul's HTTP server to ban all URLs containing non-printable characters (a bad request status will be returned for these cases). This affects some user-facing areas like key/value entry key names which are carried in URLs. [[GH-3762](https://github.com/hashicorp/consul/issues/3762)]

FEATURES:

* agent: Added retry-join support for Azure Virtual Machine Scale Sets. [[GH-3824](https://github.com/hashicorp/consul/issues/3824)]

IMPROVEMENTS:

* agent: Added agent-side telemetry around Catalog APIs to provide insight on Consul's operation from the user's perspecive. [[GH-3765](https://github.com/hashicorp/consul/issues/3765)]
* agent: Added the `NodeID` field back to the /v1/agent/self endpoint's `Config` block. [[GH-3778](https://github.com/hashicorp/consul/issues/3778)]
* api: Added missing `CheckID` and `Name` fields to API client's `AgentServiceCheck` structure so that IDs and names can be set when registering checks with services. [[GH-3788](https://github.com/hashicorp/consul/issues/3788)]

BUG FIXES:

* agent: Fixed an issue where config file symlinks were not being interpreted correctly. [[GH-3753](https://github.com/hashicorp/consul/issues/3753)]
* agent: Ignore malformed leftover service/check files and warn about them instead of refusing to start. [[GH-1221](https://github.com/hashicorp/consul/issues/1221)]
* agent: Enforce a valid port for the Serf WAN since it can't be disabled. [[GH-3817](https://github.com/hashicorp/consul/issues/3817)]
* agent: Stopped looging messages about zero RTTs when updating network coordinates since they are not harmful to the algorithm. Since we are still trying to find the root cause of these zero measurements, we added new metrics counters so these are still observable. [[GH-3789](https://github.com/hashicorp/consul/issues/3789)]
* server: Fixed a crash when POST-ing an empty body to the /v1/query endpoint. [[GH-3791](https://github.com/hashicorp/consul/issues/3791)]
* server: (Consul Enterprise) Fixed an issue where unhealthy servers were not replaced in a redundancy zone by autopilot (servers previously needed to be removed in order for a replacement to occur).
* ui: Added a URI escape around key/value keys so that it's not possible to create unexpected partial key names when entering characters like `?` inside a key. [[GH-3760](https://github.com/hashicorp/consul/issues/3760)]

## 1.0.2 (December 15, 2017)

IMPROVEMENTS:

* agent: Updated Serf to activate a new feature that resizes its internal message broadcast queue size based on the cluster size. This helps control the amount of memory used by the agent, but prevents spurious warnings about dropped messages in very large Consul clusters. The intent queue warnings have also been disabled since queue telemetry was already available and a simple fixed limit isn't applicable to all clusters, so it could cause a high rate of warnings about intent queue depth that were not useful or indicative of an actual issue. [[GH-3705](https://github.com/hashicorp/consul/issues/3705)]
* agent: Updates posener/complete library to 1.0, which allows autocomplete for flags after an equal sign, and simplifies autocomplete functions. [[GH-3646](https://github.com/hashicorp/consul/issues/3646)]

BUG FIXES:

* agent: Updated memberlist to pull in a fix for negative RTT measurements and their associated log messages about rejected coordinates. [[GH-3704](https://github.com/hashicorp/consul/issues/3704)]
* agent: Fixed an issue where node metadata specified via command line arguments overrode node metadata specified by configuration files, instead of merging as was done in versions of Consul prior to 1.0. [[GH-3716](https://github.com/hashicorp/consul/issues/3716)]
* agent: Fixed an issue with the /v1/session/create API where it wasn't possible to create a session without the `serfHealth` check. This is now possible again by including the `checks` key in the JSON body with an empty list. [[GH-3732](https://github.com/hashicorp/consul/issues/3732)]
* agent: Fixed an issue with anti-entropy syncing where checks for services with tags would cause periodic updates to the catalog, even when nothing had changed, causing the Raft index to grow slowly (~2 minutes per node per check) over time, and causing unnecessary writes and wake ups for blocking queries. [[GH-3642](https://github.com/hashicorp/consul/issues/3642)], [[GH-3259](https://github.com/hashicorp/consul/issues/3259)]
* cli: Added missing support for `-base64` option to `consul kv get` command. [[GH-3736](https://github.com/hashicorp/consul/issues/3736)]
* server: Fixed an issue with KV store tombstone tracking where bin tracking was being confused by monotonic time information carried in time stamps, resulting in many unnecessary bins. [[GH-3670](https://github.com/hashicorp/consul/issues/3670)]
* server: (Consul Enterprise) Fixed an issue with Network Segments where servers would not properly flood-join each other into all segments.
* server: Fixed an issue where it wasn't possible to disable Autopilot's dead server cleanup behavior using configuration files. [[GH-3730](https://github.com/hashicorp/consul/issues/3730)]
* server: Removed the 60 second timeout when restoring snapshots, which could cause large restores to fail on slower servers. [[GH-3326](https://github.com/hashicorp/consul/issues/3326)]
* server: Fixed a goroutine leak during keyring operations when errors are encountered. [[GH-3728](https://github.com/hashicorp/consul/issues/3728)]

## 1.0.1 (November 20, 2017)

FEATURES:

* **New Auto Join Cloud Providers:** Retry join support was added for Aliyun (Alibaba Cloud), Digital Ocean, OpenStack, and Scaleway. Instance metadata can be used with these to make it easy to form Consul clusters. [[GH-3634](https://github.com/hashicorp/consul/issues/3634)]
* **HTTP/2 Support:** If TLS is enabled on a Consul agent it will automatically negotiate to use HTTP/2 for suitably configured clients accessing the client API. This allows clients to multiplex requests over the same TCP connection, such as multiple, simultaneous blocking queries. [[GH-3657](https://github.com/hashicorp/consul/issues/3657)]

IMPROVEMENTS:

* agent: (Consul Enterprise) Added [AWS KMS support](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingKMSEncryption.html) for S3 snapshots using the snapshot agent.
* agent: Watches in the Consul agent can now be configured to invoke an HTTP endpoint instead of an executable. [[GH-3305](https://github.com/hashicorp/consul/issues/3305)]
* agent: Added a new [`-config-format`](https://www.consul.io/docs/agent/options.html#_config_format) command line option which can be set to `hcl` or `json` to specify the format of configuration files. This is useful for cases where the file name cannot be controlled in order to provide the required extension. [[GH-3620](https://github.com/hashicorp/consul/issues/3620)]
* agent: DNS recursors can now be specified as [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template) templates. [[GH-2932](https://github.com/hashicorp/consul/issues/2932)]
* agent: Serf snapshots no longer save network coordinate information. This enables recovery from errors upon agent restart. [[GH-489](https://github.com/hashicorp/serf/issues/489)]
* agent: Added defensive code to prevent out of range ping times from infecting network coordinates. Updates to the coordinate system with negative round trip times or round trip times higher than 10 seconds will log an error but will be ignored.
* agent: The agent now warns when there are extra unparsed command line arguments and refuses to start. [[GH-3397](https://github.com/hashicorp/consul/issues/3397)]
* agent: Updated go-sockaddr library to get CoreOS route detection fixes and the new `mask` functionality. [[GH-3633](https://github.com/hashicorp/consul/issues/3633)]
* agent: Added a new [`enable_agent_tls_for_checks`](https://www.consul.io/docs/agent/options.html#enable_agent_tls_for_checks) configuration option that allows HTTP health checks for services requiring 2-way TLS to be checked using the agent's credentials. [[GH-3364](https://github.com/hashicorp/consul/issues/3364)]
* agent: Made logging of health check status more uniform and moved log entries with full check output from DEBUG to TRACE level for less noise. [[GH-3683](https://github.com/hashicorp/consul/issues/3683)]
* build: Consul is now built with Go 1.9.2. [[GH-3663](https://github.com/hashicorp/consul/issues/3663)]

BUG FIXES:

* agent: Consul 1.0 shipped with an issue where `Args` was erroneously named `ScriptArgs` for health check definitions in the /v1/agent/check/register and /v1/agent/service/register APIs. Added code to accept `Args` so that the JSON format matches that of health checks in configuration files. The `ScriptArgs` form will still be supported for backwards compatibility. [[GH-3587](https://github.com/hashicorp/consul/issues/3587)]
* agent: Docker container checks running on Linux could get into a flapping state because the Docker agent seems to close the connection prematurely even though the body is transferred. This caused a "connection reset by peer" error which put the check into `critical` state. As of Consul 1.0.1 the "connection reset by peer" error is ignored for the `/exec/<execID>/start` command of the Docker API. [[GH-3576](https://github.com/hashicorp/consul/issues/3576)]
* agent: Added new form of `consul.http.*` metrics that were accidentally left out of Consul 1.0. [[GH-3654](https://github.com/hashicorp/consul/issues/3654)]
* agent: Fixed an issue with the server manager where periodic server client connection rebalancing could select a failed server. This affects agents in client mode, as well as servers talking to other servers, including over the WAN. [[GH-3463](https://github.com/hashicorp/consul/issues/3463)]
* agent: IPv6 addresses without port numbers and without surrounding brackets are now properly handled for joins. This affects all join types, but in particular this was discovered with AWS joins where the APIs return addresses formatted this way. [[GH-3671](https://github.com/hashicorp/consul/issues/3671)]
* agent: Fixed a rare startup panic of the Consul agent related to the LAN Serf instance ordering with the router manager. [[GH-3680](https://github.com/hashicorp/consul/issues/3680)]
* agent: Added back an exception for the `snapshot_agent` config key so that those configs can again live alongside Consul's configs. [[GH-3678](https://github.com/hashicorp/consul/issues/3678)]
* dns: Fixed an issue were components of a host name near the datacenter could be quietly ignored (eg. `foo.service.dc1.extra.consul` would silently ignore `.extra`); now an `NXDOMAIN` error will be returned. [[GH-3200](https://github.com/hashicorp/consul/issues/3200)]
* server: Fixed an issue where performing rolling updates of Consul servers could result in an outage from old servers remaining in the cluster. Consul's Autopilot would normally remove old servers when new ones come online, but it was also waiting to promote servers to voters in pairs to maintain an odd quorum size. The pairwise promotion feature was removed so that servers become voters as soon as they are stable, allowing Autopilot to remove old servers in a safer way. When upgrading from Consul 1.0, you may need to manually force-leave old servers as part of a rolling update to Consul 1.0.1. [[GH-3611](https://github.com/hashicorp/consul/issues/3611)]
* server: Fixed a deadlock where tombstone garbage collection for the KV store could block other KV operations, stalling writes on the leader. [[GH-3700](https://github.com/hashicorp/consul/issues/3700)]

## 1.0.0 (October 16, 2017)

SECURITY:

* ui: Fixed an XSS issue with Consul's built-in web UI where node names were not being properly escaped. [[GH-3578](https://github.com/hashicorp/consul/issues/3578)]

BREAKING CHANGES:

* **Raft Protocol Now Defaults to 3:** The [`-raft-protocol`](https://www.consul.io/docs/agent/options.html#_raft_protocol) default has been changed from 2 to 3, enabling all [Autopilot](https://www.consul.io/docs/guides/autopilot.html) features by default. Version 3 requires Consul running 0.8.0 or newer on all servers in order to work, so if you are upgrading with older servers in a cluster then you will need to set this back to 2 in order to upgrade. See [Raft Protocol Version Compatibility](https://www.consul.io/docs/upgrade-specific.html#raft-protocol-version-compatibility) for more details. Also the format of `peers.json` used for outage recovery is different when running with the lastest Raft protocol. See [Manual Recovery Using peers.json](https://www.consul.io/docs/guides/outage.html#manual-recovery-using-peers-json) for a description of the required format. [[GH-3477](https://github.com/hashicorp/consul/issues/3477)]
* **Config Files Require an Extension:** As part of supporting the [HCL](https://github.com/hashicorp/hcl#syntax) format for Consul's config files, an `.hcl` or `.json` extension is required for all config files loaded by Consul, even when using the [`-config-file`](https://www.consul.io/docs/agent/options.html#_config_file) argument to specify a file directly. [[GH-3480](https://github.com/hashicorp/consul/issues/3480)]
* **Deprecated Options Have Been Removed:** All of Consul's previously deprecated command line flags and config options have been removed, so these will need to be mapped to their equivalents before upgrading. [[GH-3480](https://github.com/hashicorp/consul/issues/3480)]

    <details><summary>Detailed List of Removed Options and their Equivalents</summary>

    | Removed Option | Equivalent |
    | -------------- | ---------- |
    | `-atlas` | None, Atlas is no longer supported. |
    | `-atlas-token`| None, Atlas is no longer supported. |
    | `-atlas-join` | None, Atlas is no longer supported. |
    | `-atlas-endpoint` | None, Atlas is no longer supported. |
    | `-dc` | [`-datacenter`](https://www.consul.io/docs/agent/options.html#_datacenter) |
    | `-retry-join-azure-tag-name` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#microsoft-azure) |
    | `-retry-join-azure-tag-value` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#microsoft-azure) |
    | `-retry-join-ec2-region` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#amazon-ec2) |
    | `-retry-join-ec2-tag-key` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#amazon-ec2) |
    | `-retry-join-ec2-tag-value` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#amazon-ec2) |
    | `-retry-join-gce-credentials-file` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `-retry-join-gce-project-name` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `-retry-join-gce-tag-name` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `-retry-join-gce-zone-pattern` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `addresses.rpc` | None, the RPC server for CLI commands is no longer supported. |
    | `advertise_addrs` | [`ports`](https://www.consul.io/docs/agent/options.html#ports) with [`advertise_addr`](https://www.consul/io/docs/agent/options.html#advertise_addr) and/or [`advertise_addr_wan`](https://www.consul.io/docs/agent/options.html#advertise_addr_wan) |
    | `atlas_infrastructure` | None, Atlas is no longer supported. |
    | `atlas_token` | None, Atlas is no longer supported. |
    | `atlas_acl_token` | None, Atlas is no longer supported. |
    | `atlas_join` | None, Atlas is no longer supported. |
    | `atlas_endpoint` | None, Atlas is no longer supported. |
    | `dogstatsd_addr` | [`telemetry.dogstatsd_addr`](https://www.consul.io/docs/agent/options.html#telemetry-dogstatsd_addr) |
    | `dogstatsd_tags` | [`telemetry.dogstatsd_tags`](https://www.consul.io/docs/agent/options.html#telemetry-dogstatsd_tags) |
    | `http_api_response_headers` | [`http_config.response_headers`](https://www.consul.io/docs/agent/options.html#response_headers) |
    | `ports.rpc` | None, the RPC server for CLI commands is no longer supported. |
    | `recursor` | [`recursors`](https://github.com/hashicorp/consul/blob/master/website/source/docs/agent/options.html.md#recursors) |
    | `retry_join_azure` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#microsoft-azure) |
    | `retry_join_ec2` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#amazon-ec2) |
    | `retry_join_gce` | [`-retry-join`](https://www.consul.io/docs/agent/options.html#google-compute-engine) |
    | `statsd_addr` | [`telemetry.statsd_address`](https://github.com/hashicorp/consul/blob/master/website/source/docs/agent/options.html.md#telemetry-statsd_address) |
    | `statsite_addr` | [`telemetry.statsite_address`](https://github.com/hashicorp/consul/blob/master/website/source/docs/agent/options.html.md#telemetry-statsite_address) |
    | `statsite_prefix` | [`telemetry.metrics_prefix`](https://www.consul.io/docs/agent/options.html#telemetry-metrics_prefix) |
    | `telemetry.statsite_prefix` | [`telemetry.metrics_prefix`](https://www.consul.io/docs/agent/options.html#telemetry-metrics_prefix) |
    | (service definitions) `serviceid` | [`service_id`](https://www.consul.io/docs/agent/services.html) |
    | (service definitions) `dockercontainerid` | [`docker_container_id`](https://www.consul.io/docs/agent/services.html) |
    | (service definitions) `tlsskipverify` | [`tls_skip_verify`](https://www.consul.io/docs/agent/services.html) |
    | (service definitions) `deregistercriticalserviceafter` | [`deregister_critical_service_after`](https://www.consul.io/docs/agent/services.html) |

    </details>

* **`statsite_prefix` Renamed to `metrics_prefix`:** Since the `statsite_prefix` configuration option applied to all telemetry providers, `statsite_prefix` was renamed to [`metrics_prefix`](https://www.consul.io/docs/agent/options.html#telemetry-metrics_prefix). Configuration files will need to be updated when upgrading to this version of Consul. [[GH-3498](https://github.com/hashicorp/consul/issues/3498)]
* **`advertise_addrs` Removed:** This configuration option was removed since it was redundant with `advertise_addr` and `advertise_addr_wan` in combination with `ports` and also wrongly stated that you could configure both host and port. [[GH-3516](https://github.com/hashicorp/consul/issues/3516)]
* **Escaping Behavior Changed for go-discover Configs:** The format for [`-retry-join`](https://www.consul.io/docs/agent/options.html#retry-join) and [`-retry-join-wan`](https://www.consul.io/docs/agent/options.html#retry-join-wan) values that use [go-discover](https://github.com/hashicorp/go-discover) cloud auto joining has changed. Values in `key=val` sequences must no longer be URL encoded and can be provided as literals as long as they do not contain spaces, backslashes `\` or double quotes `"`. If values contain these characters then use double quotes as in `"some key"="some value"`. Special characters within a double quoted string can be escaped with a backslash `\`. [[GH-3417](https://github.com/hashicorp/consul/issues/3417)]
* **HTTP Verbs are Enforced in Many HTTP APIs:** Many endpoints in the HTTP API that previously took any HTTP verb now check for specific HTTP verbs and enforce them. This may break clients relying on the old behavior. [[GH-3405](https://github.com/hashicorp/consul/issues/3405)]

    <details><summary>Detailed List of Updated Endpoints and Required HTTP Verbs</summary>

    | Endpoint | Required HTTP Verb |
    | -------- | ------------------ |
    | /v1/acl/info | GET |
    | /v1/acl/list | GET |
    | /v1/acl/replication | GET |
    | /v1/agent/check/deregister | PUT |
    | /v1/agent/check/fail | PUT |
    | /v1/agent/check/pass | PUT |
    | /v1/agent/check/register | PUT |
    | /v1/agent/check/warn | PUT |
    | /v1/agent/checks | GET |
    | /v1/agent/force-leave | PUT |
    | /v1/agent/join | PUT |
    | /v1/agent/members | GET |
    | /v1/agent/metrics | GET |
    | /v1/agent/self | GET |
    | /v1/agent/service/register | PUT |
    | /v1/agent/service/deregister | PUT |
    | /v1/agent/services | GET |
    | /v1/catalog/datacenters | GET |
    | /v1/catalog/deregister | PUT |
    | /v1/catalog/node | GET |
    | /v1/catalog/nodes | GET |
    | /v1/catalog/register | PUT |
    | /v1/catalog/service | GET |
    | /v1/catalog/services | GET |
    | /v1/coordinate/datacenters | GET |
    | /v1/coordinate/nodes | GET |
    | /v1/health/checks | GET |
    | /v1/health/node | GET |
    | /v1/health/service | GET |
    | /v1/health/state | GET |
    | /v1/internal/ui/node | GET |
    | /v1/internal/ui/nodes | GET |
    | /v1/internal/ui/services | GET |
    | /v1/session/info | GET |
    | /v1/session/list | GET |
    | /v1/session/node | GET |
    | /v1/status/leader | GET |
    | /v1/status/peers | GET |
    | /v1/operator/area/:uuid/members | GET |
    | /v1/operator/area/:uuid/join | PUT |

    </details>

* **Unauthorized KV Requests Return 403:** When ACLs are enabled, reading a key with an unauthorized token returns a 403. This previously returned a 404 response.
* **Config Section of Agent Self Endpoint has Changed:** The /v1/agent/self endpoint's `Config` section has often been in flux as it was directly returning one of Consul's internal data structures. This configuration structure has been moved under `DebugConfig`, and is documents as for debugging use and subject to change, and a small set of elements of `Config` have been maintained and documented. See [Read Configuration](https://www.consul.io/api/agent.html#read-configuration) endpoint documentation for details. [[GH-3532](https://github.com/hashicorp/consul/issues/3532)]
* **Deprecated `configtest` Command Removed:** The `configtest` command was deprecated and has been superseded by the `validate` command.
* **Undocumented Flags in `validate` Command Removed:** The `validate` command supported the `-config-file` and `-config-dir` command line flags but did not document them. This support has been removed since the flags are not required.
* **Metric Names Updated:** Metric names no longer start with `consul.consul`. To help with transitioning dashboards and other metric consumers, the field `enable_deprecated_names` has been added to the telemetry section of the config, which will enable metrics with the old naming scheme to be sent alongside the new ones. [[GH-3535](https://github.com/hashicorp/consul/issues/3535)]

    <details><summary>Detailed List of Affected Metrics by Prefix</summary>

    | Prefix |
    | ------ |
    | consul.consul.acl |
    | consul.consul.autopilot |
    | consul.consul.catalog |
    | consul.consul.fsm |
    | consul.consul.health |
    | consul.consul.http |
    | consul.consul.kvs |
    | consul.consul.leader |
    | consul.consul.prepared-query |
    | consul.consul.rpc |
    | consul.consul.session |
    | consul.consul.session_ttl |
    | consul.consul.txn |

    </details>

* **Checks Validated On Agent Startup:** Consul agents now validate health check definitions in their configuration and will fail at startup if any checks are invalid. In previous versions of Consul, invalid health checks would get skipped. [[GH-3559](https://github.com/hashicorp/consul/issues/3559)]

FEATURES:

* **Support for HCL Config Files:** Consul now supports HashiCorp's [HCL](https://github.com/hashicorp/hcl#syntax) format for config files. This is easier to work with than JSON and supports comments. As part of this change, all config files will need to have either an `.hcl` or `.json` extension in order to specify their format. [[GH-3480](https://github.com/hashicorp/consul/issues/3480)]
* **Support for Binding to Multiple Addresses:** Consul now supports binding to multiple addresses for its HTTP, HTTPS, and DNS services. You can provide a space-separated list of addresses to [`-client`](https://www.consul.io/docs/agent/options.html#_client) and [`addresses`](https://www.consul.io/docs/agent/options.html#addresses) configurations, or specify a [go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr/template) template that resolves to multiple addresses. [[GH-3480](https://github.com/hashicorp/consul/issues/3480)]
* **Support for RFC1464 DNS TXT records:** Consul DNS responses now contain the node meta data encoded according to RFC1464 as TXT records. [[GH-3343](https://github.com/hashicorp/consul/issues/3343)]
* **Support for Running Subproccesses Directly Without a Shell:** Consul agent checks and watches now support an `args` configuration which is a list of arguments to run for the subprocess, which runs the subprocess directly without a shell. The old `script` and `handler` configurations are now deprecated (specify a shell explicitly if you require one). A `-shell=false` option is also available on `consul lock`, `consul watch`, and `consul exec` to run the subprocesses associated with those without a shell. [[GH-3509](https://github.com/hashicorp/consul/issues/3509)]
* **Sentinel Integration:** (Consul Enterprise) Consul's ACL system integrates with [Sentinel](https://www.consul.io/docs/guides/sentinel.html) to enable code policies that apply to KV writes.

IMPROVEMENTS:

* agent: Added support to detect public IPv4 and IPv6 addresses on AWS. [[GH-3471](https://github.com/hashicorp/consul/issues/3471)]
* agent: Improved /v1/operator/raft/configuration endpoint which allows Consul to avoid an extra agent RPC call for the `consul operator raft list-peers` command. [[GH-3449](https://github.com/hashicorp/consul/issues/3449)]
* agent: Improved ACL system for the KV store to support list permissions. This behavior can be opted in. For more information, see the [ACL Guide](https://www.consul.io/docs/guides/acl.html#list-policy-for-keys). [[GH-3511](https://github.com/hashicorp/consul/issues/3511)]
* agent: Updates miekg/dns library to later version to pick up bug fixes and improvements. [[GH-3547](https://github.com/hashicorp/consul/issues/3547)]
* agent: Added automatic retries to the RPC path, and a brief RPC drain time when servers leave. These changes make Consul more robust during graceful leaves of Consul servers, such as during upgrades, and help shield applications from "no leader" errors. These are configured with new [`performance`](https://www.consul.io/docs/agent/options.html#performance) options. [[GH-3514](https://github.com/hashicorp/consul/issues/3514)]
* agent: Added a new `discard_check_output` agent-level configuration option that can be used to trade off write load to the Consul servers vs. visibility of health check output. This is reloadable so it can be toggled without fully restarting the agent. [[GH-3562](https://github.com/hashicorp/consul/issues/3562)]
* api: Updated the API client to ride out network errors when monitoring locks and semaphores. [[GH-3553](https://github.com/hashicorp/consul/issues/3553)]
* build: Updated Go toolchain to version 1.9.1. [[GH-3537](https://github.com/hashicorp/consul/issues/3537)]
* cli: `consul lock` and `consul watch` commands will forward `TERM` and `KILL` signals to their child subprocess. [[GH-3509](https://github.com/hashicorp/consul/issues/3509)]
* cli: Added support for [autocompletion](https://www.consul.io/docs/commands/index.html#autocompletion). [[GH-3412](https://github.com/hashicorp/consul/issues/3412)]
* server: Updated BoltDB to final version 1.3.1. [[GH-3502](https://github.com/hashicorp/consul/issues/3502)]
* server: Improved dead member reap algorithm to fix edge cases where servers could get left behind. [[GH-3452](https://github.com/hashicorp/consul/issues/3452)]

BUG FIXES:

* agent: Fixed an issue where disabling both the http and https interfaces would cause a watch-related error on agent startup, even when no watches were defined. [[GH-3425](https://github.com/hashicorp/consul/issues/3425)]
* agent: Added an additional step to kill health check scripts that timeout on all platforms except Windows, and added a wait so that it's not possible to run multiple instances of the same health check script at the same time. [[GH-3565](https://github.com/hashicorp/consul/issues/3565)]
* cli: If the `consul operator raft list-peers` command encounters an error it will now exit with a non-zero exit code. [[GH-3513](https://github.com/hashicorp/consul/issues/3513)]
* cli: CLI commands will now show help for all of their arguments. [[GH-3536](https://github.com/hashicorp/consul/issues/3536)]
* server: Fixed an issue where the leader server could get into a state where it was no longer performing the periodic leader loop duties and unable to serve consistent reads after a barrier timeout error. [[GH-3545](https://github.com/hashicorp/consul/issues/3545)]

## 0.9.3 (September 8, 2017)

FEATURES:
* **LAN Network Segments:** (Consul Enterprise) Added a new [Network Segments](https://www.consul.io/docs/guides/segments.html) capability which allows users to configure Consul to support segmented LAN topologies with multiple, distinct gossip pools. [[GH-3431](https://github.com/hashicorp/consul/issues/3431)]
* **WAN Join for Cloud Providers:** Added WAN support for retry join for cloud providers via go-discover, including Amazon AWS, Microsoft Azure, Google Cloud, and SoftLayer. This uses the same "provider" syntax supported for `-retry-join` via the `-retry-join-wan` configuration. [[GH-3406](https://github.com/hashicorp/consul/issues/3406)]
* **RPC Rate Limiter:** Consul agents in client mode have a new [`limits`](https://www.consul.io/docs/agent/options.html#limits) configuration that enables a rate limit on RPC calls the agent makes to Consul servers. [[GH-3140](https://github.com/hashicorp/consul/issues/3140)]

IMPROVEMENTS:

* agent: Switched to using a read lock for the agent's RPC dispatcher, which prevents RPC calls from getting serialized. [[GH-3376](https://github.com/hashicorp/consul/issues/3376)]
* agent: When joining a cluster, Consul now skips the unique node ID constraint for Consul members running Consul older than 0.8.5. This makes it easier to upgrade to newer versions of Consul in an existing cluster with non-unique node IDs. [[GH-3070](https://github.com/hashicorp/consul/issues/3070)]
* build: Upgraded Go version to 1.9. [[GH-3428](https://github.com/hashicorp/consul/issues/3428)]
* server: Consul servers can re-establish quorum after all of them change their IP addresses upon a restart. [[GH-1580](https://github.com/hashicorp/consul/issues/1580)]
* ui: Changed text area font family to monospace to make it easier to manage complex text blocks. [[GH-3521](https://github.com/hashicorp/consul/issues/3521)]

BUG FIXES:

* agent: Fixed an issue with consul watches not triggering when ACLs are enabled. [[GH-3392](https://github.com/hashicorp/consul/issues/3392)]
* agent: Updated memberlist library for a deadlock fix. [[GH-3396](https://github.com/hashicorp/consul/issues/3396)]
* agent: Fixed a panic when retrieving NS or SOA records on Consul clients (non-servers). This also changed the Consul server list to come from the catalog and not the agent's local state when serving these requests, so the results are consistent across a cluster. [[GH-3407](https://github.com/hashicorp/consul/issues/3407)]
* cli: Updated the CLI library to pull in a fix that prevents all subcommands from being shown when showing the agent's usage list; now just top-level commands are shown. [[GH-3448](https://github.com/hashicorp/consul/issues/3448)]
* server: Fixed an issue with Consul snapshots not saving on Windows because of errors with the `fsync` syscall. [[GH-3409](https://github.com/hashicorp/consul/issues/3409)]

## 0.9.2 (August 9, 2017)

BUG FIXES:

* agent: Fixed an issue where the old `-retry-join-{ec2,azure,gce}` command line flags were not being honored. [[GH-3384](https://github.com/hashicorp/consul/issues/3384)]
* server: Reverted the change that made unauthorized KV queries return 403 instead of 404 because it had a minor bug that affected the operation of Vault, and in addition to fixing the bug, we identified an additional case that needed to be covered. This restores the <= 0.9.0 behavior until we can get a complete fix. [[GH-2637](https://github.com/hashicorp/consul/issues/2637)]

## 0.9.1 (August 9, 2017)

FEATURES:

* **Secure ACL Token Introduction:** It's now possible to manage Consul's ACL tokens without having to place any tokens inside configuration files. This supports introduction of tokens as well as rotating. This is enabled with two new APIs:
    * A new [`/v1/agent/token`](https://www.consul.io/api/agent.html#update-acl-tokens) API allows an agent's ACL tokens to be introduced without placing them into config files, and to update them without restarting the agent. See the [ACL Guide](https://www.consul.io/docs/guides/acl.html#create-an-agent-token) for an example. This was extended to ACL replication as well, along with a new [`enable_acl_replication`](https://www.consul.io/docs/agent/options.html#enable_acl_replication) config option. [GH-3324,GH-3357]
    * A new [`/v1/acl/bootstrap`](https://www.consul.io/api/acl.html#bootstrap-acls) allows a cluster's first management token to be created without using the `acl_master_token` configuration. See the [ACL Guide](https://www.consul.io/docs/guides/acl.html#bootstrapping-acls) for an example. [[GH-3349](https://github.com/hashicorp/consul/issues/3349)]
* **Metrics Viewing Endpoint:** A new [`/v1/agent/metrics`](https://www.consul.io/api/agent.html#view-metrics) API displays the current values of internally tracked metrics. [[GH-3369](https://github.com/hashicorp/consul/issues/3369)]

IMPROVEMENTS:

* agent: Retry Join for Amazon AWS, Microsoft Azure, Google Cloud, and (new) SoftLayer is now handled through the https://github.com/hashicorp/go-discover library. With this all `-retry-join-{ec2,azure,gce}-*` parameters have been deprecated in favor of a unified configuration. See [`-retry-join`](https://www.consul.io/docs/agent/options.html#_retry_join) for details. [GH-3282,GH-3351]
* agent: Reports a more detailed error message if the LAN or WAN Serf instance fails to bind to an address. [[GH-3312](https://github.com/hashicorp/consul/issues/3312)]
* agent: Added NS records and corrected SOA records to allow Consul's DNS interface to work properly with zone delegation. [[GH-1301](https://github.com/hashicorp/consul/issues/1301)]
* agent: Added support for sending metrics with labels/tags to supported backends. [[GH-3369](https://github.com/hashicorp/consul/issues/3369)]
* agent: Added a new `prefix_filter` option in the `telemetry` config to allow fine-grained allowing/blocking the sending of certain metrics by prefix. [[GH-3369](https://github.com/hashicorp/consul/issues/3369)]
* cli: Added a `-child-exit-code` option to `consul lock` so that it propagates an error code of 2 if the child process exits with an error. [[GH-947](https://github.com/hashicorp/consul/issues/947)]
* docs: Added a new [Geo Failover Guide](https://www.consul.io/docs/guides/geo-failover.html) showing how to use prepared queries to implement geo failover policies for services. [[GH-3328](https://github.com/hashicorp/consul/issues/3328)]
* docs: Added a new [Consul with Containers Guide](https://www.consul.io/docs/guides/consul-containers.html) showing critical aspects of operating a Consul cluster that's run inside containers. [[GH-3347](https://github.com/hashicorp/consul/issues/3347)]
* server: Added a `RemoveEmptyTags` option to prepared query templates which will strip out any empty strings in the tags list before executing a query. This is useful when interpolating into tags in a way where the tag is optional, and where searching for an empty tag would yield no results from the query. [[GH-2151](https://github.com/hashicorp/consul/issues/2151)]
* server: Implemented a much faster recursive delete algorithm for the KV store. It has been benchmarked to be up to 100X faster on recursive deletes that affect millions of keys. [GH-1278, GH-3313]

BUG FIXES:

* agent: Clean up temporary files during disk write errors when persisting services and checks. [[GH-3207](https://github.com/hashicorp/consul/issues/3207)]
* agent: Fixed an issue where DNS and client bind address templates were not being parsed via the go-sockaddr library. [[GH-3322](https://github.com/hashicorp/consul/issues/3322)]
* agent: Fixed status code on all KV store operations that fail due to an ACL issue. They now return a 403 status code, rather than a 404. [[GH-2637](https://github.com/hashicorp/consul/issues/2637)]
* agent: Fixed quoting issues in script health check on Windows. [[GH-1875](https://github.com/hashicorp/consul/issues/1875)]
* agent: Fixed an issue where `consul monitor` would exit on any empty log line. [[GH-3253](https://github.com/hashicorp/consul/issues/3253)]
* server: Updated raft library to fix issue with machine crashes causing snapshot files to not get saved to disk [[GH-3362](https://github.com/hashicorp/consul/issues/3362)]

## 0.9.0 (July 20, 2017)

BREAKING CHANGES:

* agent: Added a new [`enable_script_checks`](https://www.consul.io/docs/agent/options.html#_enable_script_checks) configuration option that defaults to `false`, meaning that in order to allow an agent to run health checks that execute scripts, this will need to be configured and set to `true`. This provides a safer out-of-the-box configuration for Consul where operators must opt-in to allow script-based health checks. [[GH-3087](https://github.com/hashicorp/consul/issues/3087)]
* api: Reworked `context` support in the API client to more closely match the Go standard library, and added context support to write requests in addition to read requests. [GH-3273, GH-2992]
* ui: Since the UI is now bundled with the application we no longer provide a separate UI package for downloading. [[GH-3292](https://github.com/hashicorp/consul/issues/3292)]

FEATURES:

* agent: Added a new [`block_endpoints`](https://www.consul.io/docs/agent/options.html#block_endpoints) configuration option that allows blocking HTTP API endpoints by prefix. This allows operators to completely disallow access to specific endpoints on a given agent. [[GH-3252](https://github.com/hashicorp/consul/issues/3252)]
* cli: Added a new [`consul catalog`](https://www.consul.io/docs/commands/catalog.html) command for reading datacenters, nodes, and services from the catalog. [[GH-3204](https://github.com/hashicorp/consul/issues/3204)]
* server: (Consul Enterprise) Added a new [`consul operator area update`](https://www.consul.io/docs/commands/operator/area.html#update) command and corresponding HTTP endpoint to allow for transitioning the TLS setting of network areas at runtime. [[GH-3075](https://github.com/hashicorp/consul/issues/3075)]
* server: (Consul Enterprise) Added a new `UpgradeVersionTag` field to the Autopilot config to allow for using the migration feature to roll out configuration or cluster changes, without having to upgrade Consul itself.

IMPROVEMENTS:

* agent: (Consul Enterprise) Snapshot agent rotation uses S3's pagination API, enabling retaining more than a 100 snapshots.
* agent: Removed registration of the `consul` service from the agent since it's already handled by the leader. This means that Consul servers no longer need to have an `acl_agent_token` with write access to the `consul` service if ACLs are enabled. [[GH-3248](https://github.com/hashicorp/consul/issues/3248)]
* agent: Changed /v1/acl/clone response to 403 (from 404) when trying to clone an ACL that doesn't exist. [[GH-1113](https://github.com/hashicorp/consul/issues/1113)]
* agent: Changed the `consul exec` ACL resolution logic to use the `acl_agent_token` if it's available. This lets operators configure an `acl_agent_token` with the required `write` privilieges to the `_rexec` prefix of the KV store without giving this to the `acl_token`, which would expose those privileges to users as well. [[GH-3160](https://github.com/hashicorp/consul/issues/3160)]
* agent: Updated memberlist to get latest LAN gossip tuning based on the [Lifeguard paper published by Hashicorp Research](https://www.hashicorp.com/blog/making-gossip-more-robust-with-lifeguard/). [[GH-3287](https://github.com/hashicorp/consul/issues/3287)]
* api: Added the ability to pass in a `context` as part of the `QueryOptions` during a request. This provides a way to cancel outstanding blocking queries. [[GH-3195](https://github.com/hashicorp/consul/issues/3195)]
* api: Changed signature for "done" channels on `agent.Monitor()` and `session.RenewPeriodic` methods to make them more compatible with `context`. [[GH-3271](https://github.com/hashicorp/consul/issues/3271)]
* docs: Added a complete end-to-end example of ACL bootstrapping in the [ACL Guide](https://www.consul.io/docs/guides/acl.html#bootstrapping-acls). [[GH-3248](https://github.com/hashicorp/consul/issues/3248)]
* vendor: Updated golang.org/x/sys/unix to support IBM s390 platforms. [[GH-3240](https://github.com/hashicorp/consul/issues/3240)]
* agent: rewrote Docker health checks without using the Docker client and its dependencies. [[GH-3270](https://github.com/hashicorp/consul/issues/3270)]

BUG FIXES:

* agent: Fixed an issue where watch plans would take up to 10 minutes to close their connections and give up their file descriptors after reloading Consul. [[GH-3018](https://github.com/hashicorp/consul/issues/3018)]
* agent: (Consul Enterprise) Fixed an issue with the snapshot agent where it could get stuck trying to obtain the leader lock after an extended server outage.
* agent: Fixed HTTP health checks to allow them to set the `Host` header correctly on outgoing requests. [[GH-3203](https://github.com/hashicorp/consul/issues/3203)]
* agent: Serf snapshots can now auto recover from disk write errors without needing a restart. [[GH-1744](https://github.com/hashicorp/consul/issues/1744)]
* agent: Fixed log redacting code to properly remove tokens from log lines with ACL tokens in the URL itself: `/v1/acl/clone/:uuid`, `/v1/acl/destroy/:uuid`, `/v1/acl/info/:uuid`. [[GH-3276](https://github.com/hashicorp/consul/issues/3276)]
* agent: Fixed an issue in the Docker client where Docker checks would get EOF errors trying to connect to a volume-mounted Docker socket. [[GH-3254](https://github.com/hashicorp/consul/issues/3254)]
* agent: Fixed a crash when using Azure auto discovery. [[GH-3193](https://github.com/hashicorp/consul/issues/3193)]
* agent: Added `node` read privileges to the `acl_agent_master_token` by default so it can see all nodes, which enables it to be used with operations like `consul members`. [[GH-3113](https://github.com/hashicorp/consul/issues/3113)]
* agent: Fixed an issue where enabling [`-disable-keyring-file`](https://www.consul.io/docs/agent/options.html#_disable_keyring_file) would cause gossip encryption to be disabled. [[GH-3243](https://github.com/hashicorp/consul/issues/3243)]
* agent: Fixed a race condition where checks that are not associated with any existing services were allowed to persist. [[GH-3297](https://github.com/hashicorp/consul/issues/3297)]
* agent: Stop docker checks on service deregistration and on shutdown. [GH-3265, GH-3295]
* server: Updated the Raft library to pull in a fix where servers that are very far behind in replication can get stuck in a loop trying to install snapshots. [[GH-3201](https://github.com/hashicorp/consul/issues/3201)]
* server: Fixed a rare but serious deadlock where the Consul leader routine could get stuck with the Raft internal leader routine while waiting for the initial barrier after a leader election. [[GH-3230](https://github.com/hashicorp/consul/issues/3230)]
* server: Added automatic cleanup of failed Raft snapshots. [[GH-3258](https://github.com/hashicorp/consul/issues/3258)]
* server: (Consul Enterprise) Fixed an issue where networks areas would not be able to be added when the server restarts if the Raft log contained a specific sequence of adds and deletes for network areas with the same peer datacenter.
* ui: Provided a path to reset the ACL token when the current token is invalid. Previously, the UI would get stuck on the error page and it wasn't possible to get back to the settings. [[GH-2370](https://github.com/hashicorp/consul/issues/2370)]
* ui: Removed an extra fetch of the nodes resource when loading the UI. [[GH-3245](https://github.com/hashicorp/consul/issues/3245)]
* ui: Changed default ACL token type to "client" when creating ACLs. [[GH-3246](https://github.com/hashicorp/consul/issues/3246)]
* ui: Display a 404 error instead of a 200 when trying to load a nonexistent node. [[GH-3251](https://github.com/hashicorp/consul/issues/3251)]

## 0.8.5 (June 27, 2017)

BREAKING CHANGES:

* agent: Parse values given to `?passing` for health endpoints. Previously Consul only checked for the existence of the querystring, not the value. That means using `?passing=false` would actually still include passing values. Consul now parses the value given to passing as a boolean. If no value is provided, the old behavior remains. This may be a breaking change for some users, but the old experience was incorrect and caused enough confusion to warrant changing it. [GH-2212, GH-3136]
* agent: The default value of [`-disable-host-node-id`](https://www.consul.io/docs/agent/options.html#_disable_host_node_id) has been changed from false to true. This means you need to opt-in to host-based node IDs and by default Consul will generate a random node ID. A high number of users struggled to deploy newer versions of Consul with host-based IDs because of various edge cases of how the host IDs work in Docker, on specially-provisioned machines, etc. so changing this from opt-out to opt-in will ease operations for many Consul users. [[GH-3171](https://github.com/hashicorp/consul/issues/3171)]

IMPROVEMENTS:

* agent: Added a `-disable-keyring-file` option to prevent writing keyring data to disk. [[GH-3145](https://github.com/hashicorp/consul/issues/3145)]
* agent: Added automatic notify to systemd on Linux after LAN join is complete, which makes it easier to order services that depend on Consul being available. [[GH-2121](https://github.com/hashicorp/consul/issues/2121)]
* agent: The `http_api_response_headers` config has been moved into a new `http_config` struct, so the old form is still supported but is deprecated. [[GH-3142](https://github.com/hashicorp/consul/issues/3142)]
* dns: Added support for EDNS(0) size adjustments if set in the request frame. This allows DNS responses via UDP which are larger than the standard 512 bytes max if the requesting client can support it. [GH-1980, GH-3131]
* server: Added a startup warning for servers when expecting to bootstrap with an even number of nodes. [[GH-1282](https://github.com/hashicorp/consul/issues/1282)]
* agent: (Consul Enterprise) Added support for non rotating, statically named snapshots for S3 snapshots using the snapshot agent.

BUG FIXES:

* agent: Fixed a regression where configuring -1 for the port was no longer disabling the DNS server. [[GH-3135](https://github.com/hashicorp/consul/issues/3135)]
* agent: Fix `consul leave` shutdown race. When shutting down an agent via the `consul leave` command on the command line the output would be `EOF` instead of `Graceful leave completed` [[GH-2880](https://github.com/hashicorp/consul/issues/2880)]
* agent: Show a better error message than 'EOF' when attempting to join with the wrong gossip key. [[GH-1013](https://github.com/hashicorp/consul/issues/1013)]
* agent: Fixed an issue where the `Method` and `Header` features of HTTP health checks were not being applied. [[GH-3178](https://github.com/hashicorp/consul/issues/3178)]
* agent: Fixed an issue where internally-configured watches were not working because of an incorrect protocol error, and unified internal watch handling during reloads of the Consul agent. [[GH-3177](https://github.com/hashicorp/consul/issues/3177)]
* server: Fixed an issue where the leader could return stale data duing queries as it is starting up. [[GH-2644](https://github.com/hashicorp/consul/issues/2644)]

## 0.8.4 (June 9, 2017)

FEATURES:

* agent: Added a method for [transitioning to gossip encryption on an existing cluster](https://www.consul.io/docs/agent/encryption.html#configuring-gossip-encryption-on-an-existing-cluster). [[GH-3079](https://github.com/hashicorp/consul/issues/3079)]
* agent: Added a method for [transitioning to TLS on an existing cluster](https://www.consul.io/docs/agent/encryption.html#configuring-tls-on-an-existing-cluster). [[GH-1705](https://github.com/hashicorp/consul/issues/1705)]
* agent: Added support for [RetryJoin on Azure](https://www.consul.io/docs/agent/options.html#retry_join_azure). [[GH-2978](https://github.com/hashicorp/consul/issues/2978)]
* agent: (Consul Enterprise) Added [AWS server side encryption support](http://docs.aws.amazon.com/AmazonS3/latest/dev/UsingServerSideEncryption.html) for S3 snapshots using the snapshot agent.

IMPROVEMENTS:

* agent: Added a check which prevents advertising or setting a service to a zero address (`0.0.0.0`, `[::]`, `::`). [[GH-2961](https://github.com/hashicorp/consul/issues/2961)]
* agent: Allow binding to any public IPv6 address with `::` [[GH-2285](https://github.com/hashicorp/consul/issues/2285)]
* agent: Removed SCADA-related code for Atlas and deprecated all Atlas-related configuration options. [[GH-3032](https://github.com/hashicorp/consul/issues/3032)]
* agent: Added support for custom check id and name when registering checks along with a service. [[GH-3047](https://github.com/hashicorp/consul/issues/3047)]
* agent: Updated [go-sockaddr](https://github.com/hashicorp/go-sockaddr) library to add support for new helper functions in bind address templates (`GetPrivateIPs`, `GetPublicIPs`), new math functions, and to pick up fixes for issues with detecting addresses on multi-homed hosts. [[GH-3068](https://github.com/hashicorp/consul/issues/3068)]
* agent: Watches now reset their index back to zero after an error, or if the index goes backwards, which allows watches to recover after a server restart with fresh state. [[GH-2621](https://github.com/hashicorp/consul/issues/2621)]
* agent: HTTP health checks now support custom method and headers. [[GH-1184](https://github.com/hashicorp/consul/issues/1184)], [[GH-2474](https://github.com/hashicorp/consul/issues/2474)], [[GH-2657](https://github.com/hashicorp/consul/issues/2657)], [[GH-3106](https://github.com/hashicorp/consul/issues/3106)]
* agent: Increased the graceful leave timeout from 5 to 15 seconds. [[GH-3121](https://github.com/hashicorp/consul/issues/3121)]
* agent: Added additional logging when the agent handles signals and when it exits. [[GH-3124](https://github.com/hashicorp/consul/issues/3124)]
* build: Added support for linux/arm64 binaries. [[GH-3042](https://github.com/hashicorp/consul/issues/3042)]
* build: Consul now builds with Go 1.8.3. [[GH-3074](https://github.com/hashicorp/consul/issues/3074)]
* ui: Added a sticky scroll to the KV side panel so the KV edit box always stays in place. [[GH-2812](https://github.com/hashicorp/consul/issues/2812)]

BUG FIXES:

* agent: Added defensive code to prevent agents from infecting the network coordinates with `NaN` or `Inf` values, and added code to clean up in environments where this has happened. [[GH-3023](https://github.com/hashicorp/consul/issues/3023)]
* api: Added code to always read from the body of a request so that connections will always be returned to the pool. [[GH-2850](https://github.com/hashicorp/consul/issues/2850)]
* build: Added a vendor fix to allow compilation on Illumos. [[GH-3024](https://github.com/hashicorp/consul/issues/3024)]
* cli: Fixed an issue where `consul exec` would return a 0 exit code, even when there were nodes that didn't respond. [[GH-2757](https://github.com/hashicorp/consul/issues/2757)]

## 0.8.3 (May 12, 2017)

BUG FIXES:

* agent: Fixed an issue where NAT-configured agents with a non-routable advertise address would refuse to make RPC connections to Consul servers. This was a regression related to GH-2822 in Consul 0.8.2. [[GH-3028](https://github.com/hashicorp/consul/issues/3028)]

## 0.8.2 (May 9, 2017)

BREAKING CHANGES:

* api: HttpClient now defaults to nil in the client config and will be generated if left blank. A NewHttpClient function has been added for creating an HttpClient with a custom Transport or TLS config. [[GH-2922](https://github.com/hashicorp/consul/issues/2922)]

IMPROVEMENTS:

* agent: Added an error at agent startup time if both `-ui` and `-ui-dir` are configured together. [[GH-2576](https://github.com/hashicorp/consul/issues/2576)]
* agent: Added the datacenter of a node to the catalog, health, and query API endpoints which contain a Node structure. [[GH-2713](https://github.com/hashicorp/consul/issues/2713)]
* agent: Added the `ca_path`, `tls_cipher_suites`, and `tls_prefer_server_cipher_suites` options to give more flexibility around configuring TLS. [[GH-2963](https://github.com/hashicorp/consul/issues/2963)]
* agent: Reduced the timeouts for the `-dev` server mode so that the development server starts up almost instantly. [[GH-2984](https://github.com/hashicorp/consul/issues/2984)]
* agent: Added `verify_incoming_rpc` and `verify_incoming_https` options for more granular control over incoming TLS enforcement. [[GH-2974](https://github.com/hashicorp/consul/issues/2974)]
* agent: Use bind address as source for outgoing connections. [[GH-2822](https://github.com/hashicorp/consul/issues/2822)]
* api: Added the ACL replication status endpoint to the Go API client library. [[GH-2947](https://github.com/hashicorp/consul/issues/2947)]
* cli: Added Raft protocol version to output of `operator raft list-peers` command.[[GH-2929](https://github.com/hashicorp/consul/issues/2929)]
* ui: Added optional JSON validation when editing KV entries in the web UI. [[GH-2712](https://github.com/hashicorp/consul/issues/2712)]
* ui: Updated ACL guide links and made guides open in a new tab. [[GH-3010](https://github.com/hashicorp/consul/issues/3010)]

BUG FIXES:

* server: Fixed a panic when the tombstone garbage collector was stopped. [[GH-2087](https://github.com/hashicorp/consul/issues/2087)]
* server: Fixed a panic in Autopilot that could occur when a node is elected but cannot complete leader establishment and steps back down. [[GH-2980](https://github.com/hashicorp/consul/issues/2980)]
* server: Added a new peers.json format that allows outage recovery when using Raft protocol version 3 and higher. Previously, you'd have to set the Raft protocol version back to 2 in order to manually recover a cluster. See https://www.consul.io/docs/guides/outage.html#manual-recovery-using-peers-json for more details. [[GH-3003](https://github.com/hashicorp/consul/issues/3003)]
* ui: Add and update favicons [[GH-2945](https://github.com/hashicorp/consul/issues/2945)]

## 0.8.1 (April 17, 2017)

IMPROVEMENTS:

* agent: Node IDs derived from host information are now hashed to prevent things like common server hardware from generating IDs with a common prefix across nodes. [[GH-2884](https://github.com/hashicorp/consul/issues/2884)]
* agent: Added new `-disable-host-node-id` CLI flag and `disable_host_node_id` config option to the Consul agent to prevent it from using information from the host when generating a node ID. This will result in a random node ID, which is useful when running multiple Consul agents on the same host for testing purposes. Having this built-in eases configuring a random node ID when running in containers. [[GH-2877](https://github.com/hashicorp/consul/issues/2877)]
* agent: Removed useless "==> Caught signal: broken pipe" logging since that often results from problems sending telemetry or broken incoming client connections; operators don't need to be alerted to these. [[GH-2768](https://github.com/hashicorp/consul/issues/2768)]
* cli: Added TLS options for setting the client/CA certificates to use when communicating with Consul. These can be provided through environment variables or command line flags. [[GH-2914](https://github.com/hashicorp/consul/issues/2914)]
* build: Consul is now built with Go 1.8.1. [[GH-2888](https://github.com/hashicorp/consul/issues/2888)]
* ui: Updates Consul assets to new branding. [[GH-2898](https://github.com/hashicorp/consul/issues/2898)]

BUG FIXES:

* api: Added missing Raft index fields to AgentService and Node structures. [[GH-2882](https://github.com/hashicorp/consul/issues/2882)]
* server: Fixed an issue where flood joins would not work with IPv6 addresses. [[GH-2878](https://github.com/hashicorp/consul/issues/2878)]
* server: Fixed an issue where electing a 0.8.x leader during an upgrade would cause a panic in older servers. [[GH-2889](https://github.com/hashicorp/consul/issues/2889)]
* server: Fixed an issue where tracking of leadership changes could become incorrect when changes occurred very rapidly. This could manifest as a panic in Autopilot, but could have caused other issues with multiple leader management routines running simultaneously. [[GH-2896](https://github.com/hashicorp/consul/issues/2896)]
* server: Fixed a panic when checking ACLs on a session that doesn't exist. [[GH-2624](https://github.com/hashicorp/consul/issues/2624)]

## 0.8.0 (April 5, 2017)

BREAKING CHANGES:

* **Command-Line Interface RPC Deprecation:** The RPC client interface has been removed. All CLI commands that used RPC and the `-rpc-addr` flag to communicate with Consul have been converted to use the HTTP API and the appropriate flags for it, and the `rpc` field has been removed from the port and address binding configs. You will need to remove these fields from your config files and update any scripts that passed a custom `-rpc-addr` to the following commands: `force-leave`, `info`,  `join`, `keyring`, `leave`, `members`, `monitor`, `reload`

* **Version 8 ACLs Are Now Opt-Out:** The [`acl_enforce_version_8`](https://www.consul.io/docs/agent/options.html#acl_enforce_version_8) configuration now defaults to `true` to enable [full version 8 ACL support](https://www.consul.io/docs/internals/acl.html#version_8_acls) by default. If you are upgrading an existing cluster with ACLs enabled, you will need to set this to `false` during the upgrade on **both Consul agents and Consul servers**. Version 8 ACLs were also changed so that [`acl_datacenter`](https://www.consul.io/docs/agent/options.html#acl_datacenter) must be set on agents in order to enable the agent-side enforcement of ACLs. This makes for a smoother experience in clusters where ACLs aren't enabled at all, but where the agents would have to wait to contact a Consul server before learning that. [[GH-2844](https://github.com/hashicorp/consul/issues/2844)]

* **Remote Exec Is Now Opt-In:** The default for [`disable_remote_exec`](https://www.consul.io/docs/agent/options.html#disable_remote_exec) was changed to "true", so now operators need to opt-in to having agents support running commands remotely via [`consul exec`](/docs/commands/exec.html). [[GH-2854](https://github.com/hashicorp/consul/issues/2854)]

* **Raft Protocol Compatibility:** When upgrading to Consul 0.8.0 from a version lower than 0.7.0, users will need to
set the [`-raft-protocol`](https://www.consul.io/docs/agent/options.html#_raft_protocol) option to 1 in order to maintain backwards compatibility with the old servers during the upgrade. See [Upgrading Specific Versions](https://www.consul.io/docs/upgrade-specific.html) guide for more details.

FEATURES:

* **Autopilot:** A set of features has been added to allow for automatic operator-friendly management of Consul servers. For more information about Autopilot, see the [Autopilot Guide](https://www.consul.io/docs/guides/autopilot.html).
  - **Dead Server Cleanup:** Dead servers will periodically be cleaned up and removed from the Raft peer set, to prevent them from interfering with the quorum size and leader elections.
  - **Server Health Checking:** An internal health check has been added to track the stability of servers. The thresholds of this health check are tunable as part of the [Autopilot configuration](https://www.consul.io/docs/agent/options.html#autopilot) and the status can be viewed through the [`/v1/operator/autopilot/health`](https://www.consul.io/docs/agent/http/operator.html#autopilot-health) HTTP endpoint.
  - **New Server Stabilization:** When a new server is added to the cluster, there will be a waiting period where it must be healthy and stable for a certain amount of time before being promoted to a full, voting member. This threshold can be configured using the new [`server_stabilization_time`](https://www.consul.io/docs/agent/options.html#server_stabilization_time) setting.
  - **Advanced Redundancy:** (Consul Enterprise) A new [`-non-voting-server`](https://www.consul.io/docs/agent/options.html#_non_voting_server) option flag has been added for Consul servers to configure a server that does not participate in the Raft quorum. This can be used to add read scalability to a cluster in cases where a high volume of reads to servers are needed, but non-voting servers can be lost without causing an outage. There's also a new [`redundancy_zone_tag`](https://www.consul.io/docs/agent/options.html#redundancy_zone_tag) configuration that allows Autopilot to manage separating servers into zones for redundancy. Only one server in each zone can be a voting member at one time. This helps when Consul servers are managed with automatic replacement with a system like a resource scheduler or auto-scaling group. Extra non-voting servers in each zone will be available as hot standbys (that help with read-scaling) that can be quickly promoted into service when the voting server in a zone fails.
  - **Upgrade Orchestration:** (Consul Enterprise) Autopilot will automatically orchestrate an upgrade strategy for Consul servers where it will initially add newer versions of Consul servers as non-voters, wait for a full set of newer versioned servers to be added, and then gradually swap into service as voters and swap out older versioned servers to non-voters. This allows operators to safely bring up new servers, wait for the upgrade to be complete, and then terminate the old servers.
* **Network Areas:** (Consul Enterprise) A new capability has been added which allows operators to define network areas that join together two Consul datacenters. Unlike Consul's WAN feature, network areas use just the server RPC port for communication, and pairwise relationships can be made between arbitrary datacenters, so not all servers need to be fully connected. This allows for complex topologies among Consul datacenters like hub/spoke and more general trees. See the [Network Areas Guide](https://www.consul.io/docs/guides/areas.html) for more details.
* **WAN Soft Fail:** Request routing between servers in the WAN is now more robust by treating Serf failures as advisory but not final. This means that if there are issues between some subset of the servers in the WAN, Consul will still be able to route RPC requests as long as RPCs are actually still working. Prior to WAN Soft Fail, any datacenters having connectivity problems on the WAN would mean that all DCs might potentially stop sending RPCs to those datacenters. [[GH-2801](https://github.com/hashicorp/consul/issues/2801)]
* **WAN Join Flooding:** A new routine was added that looks for Consul servers in the LAN and makes sure that they are joined into the WAN as well. This catches up up newly-added servers onto the WAN as soon as they join the LAN, keeping them in sync automatically. [[GH-2801](https://github.com/hashicorp/consul/issues/2801)]
* **Validate command:** To provide consistency across our products, the `configtest` command has been deprecated and replaced with the `validate` command (to match Nomad and Terraform). The `configtest` command will be removed in Consul 0.9. [[GH-2732](https://github.com/hashicorp/consul/issues/2732)]

IMPROVEMENTS:

* agent: Fixed a missing case where gossip would stop flowing to dead nodes for a short while. [[GH-2722](https://github.com/hashicorp/consul/issues/2722)]
* agent: Changed agent to seed Go's random number generator. [[GH-2722](https://github.com/hashicorp/consul/issues/2722)]
* agent: Serf snapshots no longer have the executable bit set on the file. [[GH-2722](https://github.com/hashicorp/consul/issues/2722)]
* agent: Consul is now built with Go 1.8. [[GH-2752](https://github.com/hashicorp/consul/issues/2752)]
* agent: Updated aws-sdk-go version (used for EC2 auto join) for Go 1.8 compatibility. [[GH-2755](https://github.com/hashicorp/consul/issues/2755)]
* agent: User-supplied node IDs are now normalized to lower-case. [[GH-2798](https://github.com/hashicorp/consul/issues/2798)]
* agent: Added checks to enforce uniqueness of agent node IDs at cluster join time and when registering with the catalog. [[GH-2832](https://github.com/hashicorp/consul/issues/2832)]
* cli: Standardized handling of CLI options for connecting to the Consul agent. This makes sure that the same set of flags and environment variables works in all CLI commands (see https://www.consul.io/docs/commands/index.html#environment-variables). [[GH-2717](https://github.com/hashicorp/consul/issues/2717)]
* cli: Updated go-cleanhttp library for better HTTP connection handling between CLI commands and the Consul agent (tunes reuse settings). [[GH-2735](https://github.com/hashicorp/consul/issues/2735)]
* cli: The `operator raft` subcommand has had its two modes split into the `list-peers` and `remove-peer` subcommands. The old flags for these will continue to work for backwards compatibility, but will be removed in Consul 0.9.
* cli: Added an `-id` flag to the `operator raft remove-peer` command to allow removing a peer by ID. [[GH-2847](https://github.com/hashicorp/consul/issues/2847)]
* dns: Allows the `.service` tag to be optional in RFC 2782 lookups. [[GH-2690](https://github.com/hashicorp/consul/issues/2690)]
* server: Changed the internal `EnsureRegistration` RPC endpoint to prevent registering checks that aren't associated with the top-level node being registered. [[GH-2846](https://github.com/hashicorp/consul/issues/2846)]

BUG FIXES:

* agent: Fixed an issue with `consul watch` not working when http was listening on a unix socket. [[GH-2385](https://github.com/hashicorp/consul/issues/2385)]
* agent: Fixed an issue where checks and services could not sync deregister operations back to the catalog when version 8 ACL support is enabled. [[GH-2818](https://github.com/hashicorp/consul/issues/2818)]
* agent: Fixed an issue where agents could use the ACL token registered with a service when registering checks for the same service that were registered with a different ACL token. [[GH-2829](https://github.com/hashicorp/consul/issues/2829)]
* cli: Fixed `consul kv` commands not reading the `CONSUL_HTTP_TOKEN` environment variable. [[GH-2566](https://github.com/hashicorp/consul/issues/2566)]
* cli: Fixed an issue where prefixing an address with a protocol (such as 'http://' or 'https://') in `-http-addr` or `CONSUL_HTTP_ADDR` would give an error.
* cli: Fixed an issue where error messages would get printed to stdout instead of stderr. [[GH-2548](https://github.com/hashicorp/consul/issues/2548)]
* server: Fixed an issue with version 8 ACLs where servers couldn't deregister nodes from the catalog during reconciliation. [[GH-2792](https://github.com/hashicorp/consul/issues/2792)] This fix was generalized and applied to registering nodes as well. [[GH-2826](https://github.com/hashicorp/consul/issues/2826)]
* server: Fixed an issue where servers could temporarily roll back changes to a node's metadata or tagged addresses when making updates to the node's health checks. [[GH-2826](https://github.com/hashicorp/consul/issues/2826)]
* server: Fixed an issue where the service name `consul` was not subject to service ACL policies with version 8 ACLs enabled. [[GH-2816](https://github.com/hashicorp/consul/issues/2816)]

## 0.7.5 (February 15, 2017)

BUG FIXES:

* server: Fixed a rare but serious issue where Consul servers could panic when performing a large delete operation followed by a specific sequence of other updates to related parts of the state store (affects KV, sessions, prepared queries, and the catalog). [[GH-2724](https://github.com/hashicorp/consul/issues/2724)]

## 0.7.4 (February 6, 2017)

IMPROVEMENTS:

* agent: Integrated gopsutil library to use built in host UUID as node ID, if available, instead of a randomly generated UUID. This makes it easier for other applications on the same host to generate the same node ID without coordinating with Consul. [[GH-2697](https://github.com/hashicorp/consul/issues/2697)]
* agent: Added a configuration option, `tls_min_version`, for setting the minimum allowed TLS version used for the HTTP API and RPC. [[GH-2699](https://github.com/hashicorp/consul/issues/2699)]
* agent: Added a `relay-factor` option to keyring operations to allow nodes to relay their response through N randomly-chosen other nodes in the cluster. [[GH-2704](https://github.com/hashicorp/consul/issues/2704)]
* build: Consul is now built with Go 1.7.5. [[GH-2682](https://github.com/hashicorp/consul/issues/2682)]
* dns: Add ability to lookup Consul agents by either their Node ID or Node Name through the node interface (e.g. DNS `(node-id|node-name).node.consul`). [[GH-2702](https://github.com/hashicorp/consul/issues/2702)]

BUG FIXES:

* dns: Fixed an issue where SRV lookups for services on a node registered with non-IP addresses were missing the CNAME record in the additional section of the response. [[GH-2695](https://github.com/hashicorp/consul/issues/2695)]

## 0.7.3 (January 26, 2017)

FEATURES:

* **KV Import/Export CLI:** `consul kv export` and `consul kv import` can be used to move parts of the KV tree between disconnected consul clusters, using JSON as the intermediate representation. [[GH-2633](https://github.com/hashicorp/consul/issues/2633)]
* **Node Metadata:** Support for assigning user-defined metadata key/value pairs to nodes has been added. This can be viewed when looking up node info, and can be used to filter the results of various catalog and health endpoints. For more information, see the [Catalog](https://www.consul.io/docs/agent/http/catalog.html), [Health](https://www.consul.io/docs/agent/http/health.html), and [Prepared Query](https://www.consul.io/docs/agent/http/query.html) endpoint documentation, as well as the [Node Meta](https://www.consul.io/docs/agent/options.html#_node_meta) section of the agent configuration. [[GH-2654](https://github.com/hashicorp/consul/issues/2654)]
* **Node Identifiers:** Consul agents can now be configured with a unique identifier, or they will generate one at startup that will persist across agent restarts. This identifier is designed to represent a node across all time, even if the name or address of the node changes. Identifiers are currently only exposed in node-related endpoints, but they will be used in future versions of Consul to help manage Consul servers and the Raft quorum in a more robust manner, as the quorum is currently tracked via addresses, which can change. [[GH-2661](https://github.com/hashicorp/consul/issues/2661)]
* **Improved Blocking Queries:** Consul's [blocking query](https://www.consul.io/api/index.html#blocking-queries) implementation was improved to provide a much more fine-grained mechanism for detecting changes. For example, in previous versions of Consul blocking to wait on a change to a specific service would result in a wake up if any service changed. Now, wake ups are scoped to the specific service being watched, if possible. This support has been added to all endpoints that support blocking queries, nothing new is required to take advantage of this feature. [[GH-2671](https://github.com/hashicorp/consul/issues/2671)]
* **GCE auto-discovery:** New `-retry-join-gce` configuration options added to allow bootstrapping by automatically discovering Google Cloud instances with a given tag at startup. [[GH-2570](https://github.com/hashicorp/consul/issues/2570)]

IMPROVEMENTS:

* build: Consul is now built with Go 1.7.4. [[GH-2676](https://github.com/hashicorp/consul/issues/2676)]
* cli: `consul kv get` now has a `-base64` flag to base 64 encode the value. [[GH-2631](https://github.com/hashicorp/consul/issues/2631)]
* cli: `consul kv put` now has a `-base64` flag for setting values which are base 64 encoded. [[GH-2632](https://github.com/hashicorp/consul/issues/2632)]
* ui: Added a notice that JS is required when viewing the web UI with JS disabled. [[GH-2636](https://github.com/hashicorp/consul/issues/2636)]

BUG FIXES:

* agent: Redacted the AWS access key and secret key ID from the /v1/agent/self output so they are not disclosed. [[GH-2677](https://github.com/hashicorp/consul/issues/2677)]
* agent: Fixed a rare startup panic due to a Raft/Serf race condition. [[GH-1899](https://github.com/hashicorp/consul/issues/1899)]
* cli: Fixed a panic when an empty quoted argument was given to `consul kv put`. [[GH-2635](https://github.com/hashicorp/consul/issues/2635)]
* tests: Fixed a race condition with check mock's map usage. [[GH-2578](https://github.com/hashicorp/consul/issues/2578)]

## 0.7.2 (December 19, 2016)

FEATURES:

* **Keyring API:** A new `/v1/operator/keyring` HTTP endpoint was added that allows for performing operations such as list, install, use, and remove on the encryption keys in the gossip keyring. See the [Keyring Endpoint](https://www.consul.io/docs/agent/http/operator.html#keyring) for more details. [[GH-2509](https://github.com/hashicorp/consul/issues/2509)]
* **Monitor API:** A new `/v1/agent/monitor` HTTP endpoint was added to allow for viewing streaming log output from the agent, similar to the `consul monitor` command. See the [Monitor Endpoint](https://www.consul.io/docs/agent/http/agent.html#agent_monitor) for more details. [[GH-2511](https://github.com/hashicorp/consul/issues/2511)]
* **Reload API:** A new `/v1/agent/reload` HTTP endpoint was added for triggering a reload of the agent's configuration. See the [Reload Endpoint](https://www.consul.io/docs/agent/http/agent.html#agent_reload) for more details. [[GH-2516](https://github.com/hashicorp/consul/issues/2516)]
* **Leave API:** A new `/v1/agent/leave` HTTP endpoint was added for causing an agent to gracefully shutdown and leave the cluster (previously, only `force-leave` was present in the HTTP API). See the [Leave Endpoint](https://www.consul.io/docs/agent/http/agent.html#agent_leave) for more details. [[GH-2516](https://github.com/hashicorp/consul/issues/2516)]
* **Bind Address Templates (beta):** Consul agents now allow [go-sockaddr/template](https://godoc.org/github.com/hashicorp/go-sockaddr/template) syntax to be used for any bind address configuration (`advertise_addr`, `bind_addr`, `client_addr`, and others). This allows for easy creation of immutable images for Consul that can fetch their own address based on an interface name, network CIDR, address family from an actual RFC number, and many other possible schemes. This feature is in beta and we may tweak the template syntax before final release, but we encourage the community to try this and provide feedback. [[GH-2563](https://github.com/hashicorp/consul/issues/2563)]
* **Complete ACL Coverage (beta):** Consul 0.8 will feature complete ACL coverage for all of Consul. To ease the transition to the new policies, a beta version of complete ACL support was added to help with testing and migration to the new features. Please see the [ACLs Internals Guide](https://www.consul.io/docs/internals/acl.html#version_8_acls) for more details. [GH-2594, GH-2592, GH-2590]

IMPROVEMENTS:

* agent: Defaults to `?pretty` JSON for HTTP API requests when in `-dev` mode. [[GH-2518](https://github.com/hashicorp/consul/issues/2518)]
* agent: Updated Circonus metrics library and added new Circonus configration options for Consul for customizing check display name and tags. [[GH-2555](https://github.com/hashicorp/consul/issues/2555)]
* agent: Added a checksum to UDP gossip messages to guard against packet corruption. [[GH-2574](https://github.com/hashicorp/consul/issues/2574)]
* agent: Check whether a snapshot needs to be taken more often (every 5 seconds instead of 2 minutes) to keep the raft file smaller and to avoid doing huge truncations when writing lots of entries very quickly. [[GH-2591](https://github.com/hashicorp/consul/issues/2591)]
* agent: Allow gossiping to suspected/recently dead nodes. [[GH-2593](https://github.com/hashicorp/consul/issues/2593)]
* agent: Changed the gossip suspicion timeout to grow smoothly as the number of nodes grows. [[GH-2593](https://github.com/hashicorp/consul/issues/2593)]
* agent: Added a deprecation notice for Atlas features to the CLI and docs. [[GH-2597](https://github.com/hashicorp/consul/issues/2597)]
* agent: Give a better error message when the given data-dir is not a directory. [[GH-2529](https://github.com/hashicorp/consul/issues/2529)]

BUG FIXES:

* agent: Fixed a panic when SIGPIPE signal was received. [[GH-2404](https://github.com/hashicorp/consul/issues/2404)]
* api: Added missing Raft index fields to `CatalogService` structure. [[GH-2366](https://github.com/hashicorp/consul/issues/2366)]
* api: Added missing notes field to `AgentServiceCheck` structure. [[GH-2336](https://github.com/hashicorp/consul/issues/2336)]
* api: Changed type of `AgentServiceCheck.TLSSkipVerify` from `string` to `bool`. [[GH-2530](https://github.com/hashicorp/consul/issues/2530)]
* api: Added new `HealthChecks.AggregatedStatus()` method that makes it easy get an overall health status from a list of checks. [[GH-2544](https://github.com/hashicorp/consul/issues/2544)]
* api: Changed type of `KVTxnOp.Verb` from `string` to `KVOp`. [[GH-2531](https://github.com/hashicorp/consul/issues/2531)]
* cli: Fixed an issue with the `consul kv put` command where a negative value would be interpreted as an argument to read from standard input. [[GH-2526](https://github.com/hashicorp/consul/issues/2526)]
* ui: Fixed an issue where extra commas would be shown around service tags. [[GH-2340](https://github.com/hashicorp/consul/issues/2340)]
* ui: Customized Bootstrap config to avoid missing font file references. [[GH-2485](https://github.com/hashicorp/consul/issues/2485)]
* ui: Removed "Deregister" button as removing nodes from the catalog isn't a common operation and leads to lots of user confusion. [[GH-2541](https://github.com/hashicorp/consul/issues/2541)]

## 0.7.1 (November 10, 2016)

BREAKING CHANGES:

* Child process reaping support has been removed, along with the `reap` configuration option. Reaping is also done via [dumb-init](https://github.com/Yelp/dumb-init) in the [Consul Docker image](https://github.com/hashicorp/docker-consul), so removing it from Consul itself simplifies the code and eases future maintainence for Consul. If you are running Consul as PID 1 in a container you will need to arrange for a wrapper process to reap child processes. [[GH-1988](https://github.com/hashicorp/consul/issues/1988)]
* The default for `max_stale` has been increased to a near-indefinite threshold (10 years) to allow DNS queries to continue to be served in the event of a long outage with no leader. A new telemetry counter has also been added at `consul.dns.stale_queries` to track when agents serve DNS queries that are over a certain staleness (>5 seconds). [[GH-2481](https://github.com/hashicorp/consul/issues/2481)]
* The api package's `PreparedQuery.Delete()` method now takes `WriteOptions` instead of `QueryOptions`. [[GH-2417](https://github.com/hashicorp/consul/issues/2417)]

FEATURES:

* **Key/Value Store Command Line Interface:** New `consul kv` commands were added for easy access to all basic key/value store operations. [[GH-2360](https://github.com/hashicorp/consul/issues/2360)]
* **Snapshot/Restore:** A new /v1/snapshot HTTP endpoint and corresponding set of `consul snapshot` commands were added for easy point-in-time snapshots for disaster recovery. Snapshots include all state managed by Consul's Raft [consensus protocol](/docs/internals/consensus.html), including Key/Value Entries, Service Catalog, Prepared Queries, Sessions, and ACLs. Snapshots can be restored on the fly into a completely fresh cluster. [[GH-2396](https://github.com/hashicorp/consul/issues/2396)]
* **AWS auto-discovery:** New `-retry-join-ec2` configuration options added to allow bootstrapping by automatically discovering AWS instances with a given tag key/value at startup. [[GH-2459](https://github.com/hashicorp/consul/issues/2459)]

IMPROVEMENTS:

* api: All session options can now be set when using `api.Lock()`. [[GH-2372](https://github.com/hashicorp/consul/issues/2372)]
* agent: Added the ability to bind Serf WAN and LAN to different interfaces than the general bind address. [[GH-2007](https://github.com/hashicorp/consul/issues/2007)]
* agent: Added a new `tls_skip_verify` configuration option for HTTP checks. [[GH-1984](https://github.com/hashicorp/consul/issues/1984)]
* build: Consul is now built with Go 1.7.3. [[GH-2281](https://github.com/hashicorp/consul/issues/2281)]

BUG FIXES:

* agent: Fixed a Go race issue with log buffering at startup. [[GH-2262](https://github.com/hashicorp/consul/issues/2262)]
* agent: Fixed a panic during anti-entropy sync for services and checks. [[GH-2125](https://github.com/hashicorp/consul/issues/2125)]
* agent: Fixed an issue on Windows where "wsarecv" errors were logged when CLI commands accessed the RPC interface. [[GH-2356](https://github.com/hashicorp/consul/issues/2356)]
* agent: Syslog initialization will now retry on errors for up to 60 seconds to avoid a race condition at system startup. [[GH-1610](https://github.com/hashicorp/consul/issues/1610)]
* agent: Fixed a panic when both -dev and -bootstrap-expect flags were provided. [[GH-2464](https://github.com/hashicorp/consul/issues/2464)]
* agent: Added a retry with backoff when a session fails to invalidate after expiring. [[GH-2435](https://github.com/hashicorp/consul/issues/2435)]
* agent: Fixed an issue where Consul would fail to start because of leftover malformed check/service state files. [[GH-1221](https://github.com/hashicorp/consul/issues/1221)]
* agent: Fixed agent crashes on macOS Sierra by upgrading Go. [GH-2407, GH-2281]
* agent: Log a warning instead of success when attempting to deregister a nonexistent service. [[GH-2492](https://github.com/hashicorp/consul/issues/2492)]
* api: Trim leading slashes from keys/prefixes when querying KV endpoints to avoid a bug with redirects in Go 1.7 (golang/go#4800). [[GH-2403](https://github.com/hashicorp/consul/issues/2403)]
* dns: Fixed external services that pointed to consul addresses (CNAME records) not resolving to A-records. [[GH-1228](https://github.com/hashicorp/consul/issues/1228)]
* dns: Fixed an issue with SRV lookups where the service address was different from the node's. [[GH-832](https://github.com/hashicorp/consul/issues/832)]
* dns: Fixed an issue where truncated records from a recursor query were improperly reported as errors. [[GH-2384](https://github.com/hashicorp/consul/issues/2384)]
* server: Fixed the port numbers in the sample JSON inside peers.info. [[GH-2391](https://github.com/hashicorp/consul/issues/2391)]
* server: Squashes ACL datacenter name to lower case and checks for proper formatting at startup. [GH-2059, GH-1778, GH-2478]
* ui: Fixed an XSS issue with the display of sessions and ACLs in the web UI. [[GH-2456](https://github.com/hashicorp/consul/issues/2456)]

## 0.7.0 (September 14, 2016)

BREAKING CHANGES:

* The default behavior of `leave_on_terminate` and `skip_leave_on_interrupt` are now dependent on whether or not the agent is acting as a server or client. When Consul is started as a server the defaults for these are `false` and `true`, respectively, which means that you have to explicitly configure a server to leave the cluster. When Consul is started as a client the defaults are the opposite, which means by default, clients will leave the cluster if shutdown or interrupted. [[GH-1909](https://github.com/hashicorp/consul/issues/1909)] [[GH-2320](https://github.com/hashicorp/consul/issues/2320)]
* The `allow_stale` configuration for DNS queries to the Consul agent now defaults to `true`, allowing for better utilization of available Consul servers and higher throughput at the expense of weaker consistency. This is almost always an acceptable tradeoff for DNS queries, but this can be reconfigured to use the old default behavior if desired. [[GH-2315](https://github.com/hashicorp/consul/issues/2315)]
* Output from HTTP checks is truncated to 4k when stored on the servers, similar to script check output. [[GH-1952](https://github.com/hashicorp/consul/issues/1952)]
* Consul's Go API client will now send ACL tokens using HTTP headers instead of query parameters, requiring Consul 0.6.0 or later. [[GH-2233](https://github.com/hashicorp/consul/issues/2233)]
* Removed support for protocol version 1, so Consul 0.7 is no longer compatible with Consul versions prior to 0.3. [[GH-2259](https://github.com/hashicorp/consul/issues/2259)]
* The Raft peers information in `consul info` has changed format and includes information about the suffrage of a server, which will be used in future versions of Consul. [[GH-2222](https://github.com/hashicorp/consul/issues/2222)]
* New [`translate_wan_addrs`](https://www.consul.io/docs/agent/options.html#translate_wan_addrs) behavior from [[GH-2118](https://github.com/hashicorp/consul/issues/2118)] translates addresses in HTTP responses and could break clients that are expecting local addresses. A new `X-Consul-Translate-Addresses` header was added to allow clients to detect if translation is enabled for HTTP responses, and a "lan" tag was added to `TaggedAddresses` for clients that need the local address regardless of translation. [[GH-2280](https://github.com/hashicorp/consul/issues/2280)]
* The behavior of the `peers.json` file is different in this version of Consul. This file won't normally be present and is used only during outage recovery. Be sure to read the updated [Outage Recovery Guide](https://www.consul.io/docs/guides/outage.html) for details. [[GH-2222](https://github.com/hashicorp/consul/issues/2222)]
* Consul's default Raft timing is now set to work more reliably on lower-performance servers, which allows small clusters to use lower cost compute at the expense of reduced performance for failed leader detection and leader elections. You will need to configure Consul to get the same performance as before. See the new [Server Performance](https://www.consul.io/docs/guides/performance.html) guide for more details. [[GH-2303](https://github.com/hashicorp/consul/issues/2303)]

FEATURES:

* **Transactional Key/Value API:** A new `/v1/txn` API was added that allows for atomic updates to and fetches from multiple entries in the key/value store inside of an atomic transaction. This includes conditional updates based on obtaining locks, and all other key/value store operations. See the [Key/Value Store Endpoint](https://www.consul.io/docs/agent/http/kv.html#txn) for more details. [[GH-2028](https://github.com/hashicorp/consul/issues/2028)]
* **Native ACL Replication:** Added a built-in full replication capability for ACLs. Non-ACL datacenters can now replicate the complete ACL set locally to their state store and fall back to that if there's an outage. Additionally, this provides a good way to make a backup ACL datacenter, or to migrate the ACL datacenter to a different one. See the [ACL Internals Guide](https://www.consul.io/docs/internals/acl.html#replication) for more details. [[GH-2237](https://github.com/hashicorp/consul/issues/2237)]
* **Server Connection Rebalancing:** Consul agents will now periodically reconnect to available Consul servers in order to redistribute their RPC query load. Consul clients will, by default, attempt to establish a new connection every 120s to 180s unless the size of the cluster is sufficiently large. The rate at which agents begin to query new servers is proportional to the size of the Consul cluster (servers should never receive more than 64 new connections per second per Consul server as a result of rebalancing). Clusters in stable environments who use `allow_stale` should see a more even distribution of query load across all of their Consul servers. [[GH-1743](https://github.com/hashicorp/consul/issues/1743)]
* **Raft Updates and Consul Operator Interface:** This version of Consul upgrades to "stage one" of the v2 HashiCorp Raft library. This version offers improved handling of cluster membership changes and recovery after a loss of quorum. This version also provides a foundation for new features that will appear in future Consul versions once the remainder of the v2 library is complete. [[GH-2222](https://github.com/hashicorp/consul/issues/2222)] <br> Consul's default Raft timing is now set to work more reliably on lower-performance servers, which allows small clusters to use lower cost compute at the expense of reduced performance for failed leader detection and leader elections. You will need to configure Consul to get the same performance as before. See the new [Server Performance](https://www.consul.io/docs/guides/performance.html) guide for more details. [[GH-2303](https://github.com/hashicorp/consul/issues/2303)] <br> Servers will now abort bootstrapping if they detect an existing cluster with configured Raft peers. This will help prevent safe but spurious leader elections when introducing new nodes with `bootstrap_expect` enabled into an existing cluster. [[GH-2319](https://github.com/hashicorp/consul/issues/2319)] <br> Added new `consul operator` command, HTTP endpoint, and associated ACL to allow Consul operators to view and update the Raft configuration. This allows a stale server to be removed from the Raft peers without requiring downtime and peers.json recovery file use. See the new [Consul Operator Command](https://www.consul.io/docs/commands/operator.html) and the [Consul Operator Endpoint](https://www.consul.io/docs/agent/http/operator.html) for details, as well as the updated [Outage Recovery Guide](https://www.consul.io/docs/guides/outage.html). [[GH-2312](https://github.com/hashicorp/consul/issues/2312)]
* **Serf Lifeguard Updates:** Implemented a new set of feedback controls for the gossip layer that help prevent degraded nodes that can't meet the soft real-time requirements from erroneously causing `serfHealth` flapping in other, healthy nodes. This feature tunes itself automatically and requires no configuration. [[GH-2101](https://github.com/hashicorp/consul/issues/2101)]
* **Prepared Query Near Parameter:** Prepared queries support baking in a new `Near` sorting parameter. This allows results to be sorted by network round trip time based on a static node, or based on the round trip time from the Consul agent where the request originated. This can be used to find a co-located service instance is one is available, with a transparent fallback to the next best alternate instance otherwise. [[GH-2137](https://github.com/hashicorp/consul/issues/2137)]
* **Automatic Service Deregistration:** Added a new `deregister_critical_service_after` timeout field for health checks which will cause the service associated with that check to get deregistered if the check is critical for longer than the timeout. This is useful for cleanup of health checks registered natively by applications, or in other situations where services may not always be cleanly shutdown. [[GH-679](https://github.com/hashicorp/consul/issues/679)]
* **WAN Address Translation Everywhere:** Extended the [`translate_wan_addrs`](https://www.consul.io/docs/agent/options.html#translate_wan_addrs) config option to also translate node addresses in HTTP responses, making it easy to use this feature from non-DNS clients. [[GH-2118](https://github.com/hashicorp/consul/issues/2118)]
* **RPC Retries:** Consul will now retry RPC calls that result in "no leader" errors for up to 5 seconds. This allows agents to ride out leader elections with a delayed response vs. an error. [[GH-2175](https://github.com/hashicorp/consul/issues/2175)]
* **Circonus Telemetry Support:** Added support for Circonus as a telemetry destination. [[GH-2193](https://github.com/hashicorp/consul/issues/2193)]

IMPROVEMENTS:

* agent: Reap time for failed nodes is now configurable via new `reconnect_timeout` and `reconnect_timeout_wan` config options ([use with caution](https://www.consul.io/docs/agent/options.html#reconnect_timeout)). [[GH-1935](https://github.com/hashicorp/consul/issues/1935)]
* agent: Joins based on a DNS lookup will use TCP and attempt to join with the full list of returned addresses. [[GH-2101](https://github.com/hashicorp/consul/issues/2101)]
* agent: Consul will now refuse to start with a helpful message if the same UNIX socket is used for more than one listening endpoint. [[GH-1910](https://github.com/hashicorp/consul/issues/1910)]
* agent: Removed an obsolete warning message when Consul starts on Windows. [[GH-1920](https://github.com/hashicorp/consul/issues/1920)]
* agent: Defaults bind address to 127.0.0.1 when running in `-dev` mode. [[GH-1878](https://github.com/hashicorp/consul/issues/1878)]
* agent: Added version information to the log when Consul starts up. [[GH-1404](https://github.com/hashicorp/consul/issues/1404)]
* agent: Added timing metrics for HTTP requests in the form of `consul.http.<verb>.<path>`. [[GH-2256](https://github.com/hashicorp/consul/issues/2256)]
* build: Updated all vendored dependencies. [[GH-2258](https://github.com/hashicorp/consul/issues/2258)]
* build: Consul releases are now built with Go 1.6.3. [[GH-2260](https://github.com/hashicorp/consul/issues/2260)]
* checks: Script checks now support an optional `timeout` parameter. [[GH-1762](https://github.com/hashicorp/consul/issues/1762)]
* checks: HTTP health checks limit saved output to 4K to avoid performance issues. [[GH-1952](https://github.com/hashicorp/consul/issues/1952)]
* cli: Added a `-stale` mode for watchers to allow them to pull data from any Consul server, not just the leader. [[GH-2045](https://github.com/hashicorp/consul/issues/2045)] [[GH-917](https://github.com/hashicorp/consul/issues/917)]
* dns: Consul agents can now limit the number of UDP answers returned via the DNS interface. The default number of UDP answers is `3`, however by adjusting the `dns_config.udp_answer_limit` configuration parameter, it is now possible to limit the results down to `1`. This tunable provides environments where RFC3484 section 6, rule 9 is enforced with an important workaround in order to preserve the desired behavior of randomized DNS results. Most modern environments will not need to adjust this setting as this RFC was made obsolete by RFC 6724\. See the [agent options](https://www.consul.io/docs/agent/options.html#udp_answer_limit) documentation for additional details for when this should be used. [[GH-1712](https://github.com/hashicorp/consul/issues/1712)]
* dns: Consul now compresses all DNS responses by default. This prevents issues when recursing records that were originally compressed, where Consul would sometimes generate an invalid, uncompressed response that was too large. [[GH-2266](https://github.com/hashicorp/consul/issues/2266)]
* dns: Added a new `recursor_timeout` configuration option to set the timeout for Consul's internal DNS client that's used for recursing queries to upstream DNS servers. [[GH-2321](https://github.com/hashicorp/consul/issues/2321)]
* dns: Added a new `-dns-port` command line option so this can be set without a config file. [[GH-2263](https://github.com/hashicorp/consul/issues/2263)]
* ui: Added a new network tomography visualization to the UI. [[GH-2046](https://github.com/hashicorp/consul/issues/2046)]

BUG FIXES:

* agent: Fixed an issue where a health check's output never updates if the check status doesn't change after the Consul agent starts. [[GH-1934](https://github.com/hashicorp/consul/issues/1934)]
* agent: External services can now be registered with ACL tokens. [[GH-1738](https://github.com/hashicorp/consul/issues/1738)]
* agent: Fixed an issue where large events affecting many nodes could cause infinite intent rebroadcasts, leading to many log messages about intent queue overflows. [[GH-1062](https://github.com/hashicorp/consul/issues/1062)]
* agent: Gossip encryption keys are now validated before being made persistent in the keyring, avoiding delayed feedback at runtime. [[GH-1299](https://github.com/hashicorp/consul/issues/1299)]
* dns: Fixed an issue where DNS requests for SRV records could be incorrectly trimmed, resulting in an ADDITIONAL section that was out of sync with the ANSWER. [[GH-1931](https://github.com/hashicorp/consul/issues/1931)]
* dns: Fixed two issues where DNS requests for SRV records on a prepared query that failed over would report the wrong domain and fail to translate addresses. [[GH-2218](https://github.com/hashicorp/consul/issues/2218)] [[GH-2220](https://github.com/hashicorp/consul/issues/2220)]
* server: Fixed a deadlock related to sorting the list of available datacenters by round trip time. [[GH-2130](https://github.com/hashicorp/consul/issues/2130)]
* server: Fixed an issue with the state store's immutable radix tree that would prevent it from using cached modified objects during transactions, leading to extra copies and increased memory / GC pressure. [[GH-2106](https://github.com/hashicorp/consul/issues/2106)]
* server: Upgraded Bolt DB to v1.2.1 to fix an issue on Windows where Consul would sometimes fail to start due to open user-mapped sections. [[GH-2203](https://github.com/hashicorp/consul/issues/2203)]

OTHER CHANGES:

* build: Switched from Godep to govendor. [[GH-2252](https://github.com/hashicorp/consul/issues/2252)]

## 0.6.4 (March 16, 2016)

BACKWARDS INCOMPATIBILITIES:

* Added a new `query` ACL type to manage prepared query names, and stopped capturing
  ACL tokens by default when prepared queries are created. This won't affect existing
  queries and how they are executed, but this will affect how they are managed. Now
  management of prepared queries can be delegated within an organization. If you use
  prepared queries, you'll need to read the
  [Consul 0.6.4 upgrade instructions](https://www.consul.io/docs/upgrade-specific.html)
  before upgrading to this version of Consul. [[GH-1748](https://github.com/hashicorp/consul/issues/1748)]
* Consul's Go API client now pools connections by default, and requires you to manually
  opt-out of this behavior. Previously, idle connections were supported and their
  lifetime was managed by a finalizer, but this wasn't reliable in certain situations.
  If you reuse an API client object during the lifetime of your application, then there's
  nothing to do. If you have short-lived API client objects, you may need to configure them
  using the new `api.DefaultNonPooledConfig()` method to avoid leaking idle connections. [[GH-1825](https://github.com/hashicorp/consul/issues/1825)]
* Consul's Go API client's `agent.UpdateTTL()` function was updated in a way that will
  only work with Consul 0.6.4 and later. The `agent.PassTTL()`, `agent.WarnTTL()`, and
  `agent.FailTTL()` functions were not affected and will continue work with older
  versions of Consul. [[GH-1794](https://github.com/hashicorp/consul/issues/1794)]

FEATURES:

* Added new template prepared queries which allow you to define a prefix (possibly even
  an empty prefix) to apply prepared query features like datacenter failover to multiple
  services with a single query definition. This makes it easy to apply a common policy to
  multiple services without having to manage many prepared queries. See
  [Prepared Query Templates](https://www.consul.io/docs/agent/http/query.html#templates)
  for more details. [[GH-1764](https://github.com/hashicorp/consul/issues/1764)]
* Added a new ability to translate address lookups when doing queries of nodes in
  remote datacenters via DNS using a new `translate_wan_addrs` configuration
  option. This allows the node to be reached within its own datacenter using its
  local address, and reached from other datacenters using its WAN address, which is
  useful in hybrid setups with mixed networks. [[GH-1698](https://github.com/hashicorp/consul/issues/1698)]

IMPROVEMENTS:

* Added a new `disable_hostname` configuration option to control whether Consul's
  runtime telemetry gets prepended with the host name. All of the telemetry
  configuration has also been moved to a `telemetry` nested structure, but the old
  format is currently still supported. [[GH-1284](https://github.com/hashicorp/consul/issues/1284)]
* Consul's Go dependencies are now vendored using Godep. [[GH-1714](https://github.com/hashicorp/consul/issues/1714)]
* Added support for `EnableTagOverride` for the catalog in the Go API client. [[GH-1726](https://github.com/hashicorp/consul/issues/1726)]
* Consul now ships built from Go 1.6. [[GH-1735](https://github.com/hashicorp/consul/issues/1735)]
* Added a new `/v1/agent/check/update/<check id>` API for updating TTL checks which
  makes it easier to send large check output as part of a PUT body and not a query
  parameter. [[GH-1785](https://github.com/hashicorp/consul/issues/1785)].
* Added a default set of `Accept` headers for HTTP checks. [[GH-1819](https://github.com/hashicorp/consul/issues/1819)]
* Added support for RHEL7/Systemd in Terraform example. [[GH-1629](https://github.com/hashicorp/consul/issues/1629)]

BUG FIXES:

* Updated the internal web UI (`-ui` option) to latest released build, fixing
  an ACL-related issue and the broken settings icon. [[GH-1619](https://github.com/hashicorp/consul/issues/1619)]
* Fixed an issue where blocking KV reads could miss updates and return stale data
  when another key whose name is a prefix of the watched key was updated. [[GH-1632](https://github.com/hashicorp/consul/issues/1632)]
* Fixed the redirect from `/` to `/ui` when the internal web UI (`-ui` option) is
  enabled. [[GH-1713](https://github.com/hashicorp/consul/issues/1713)]
* Updated memberlist to pull in a fix for leaking goroutines when performing TCP
  fallback pings. This affected users with frequent UDP connectivity problems. [[GH-1802](https://github.com/hashicorp/consul/issues/1802)]
* Added a fix to trim UDP DNS responses so they don't exceed 512 bytes. [[GH-1813](https://github.com/hashicorp/consul/issues/1813)]
* Updated go-dockerclient to fix Docker health checks with Docker 1.10. [[GH-1706](https://github.com/hashicorp/consul/issues/1706)]
* Removed fixed height display of nodes and services in UI, leading to broken displays
  when a node has a lot of services. [[GH-2055](https://github.com/hashicorp/consul/issues/2055)]

## 0.6.3 (January 15, 2016)

BUG FIXES:

* Fixed an issue when running Consul as PID 1 in a Docker container where
  it could consume CPU and show spurious failures for health checks, watch
  handlers, and `consul exec` commands [[GH-1592](https://github.com/hashicorp/consul/issues/1592)]

## 0.6.2 (January 13, 2016)

SECURITY:

* Build against Go 1.5.3 to mitigate a security vulnerability introduced
  in Go 1.5. For more information, please see https://groups.google.com/forum/#!topic/golang-dev/MEATuOi_ei4

This is a security-only release; other than the version number and building
against Go 1.5.3, there are no changes from 0.6.1.

## 0.6.1 (January 6, 2016)

BACKWARDS INCOMPATIBILITIES:

* The new `-monitor-retry` option to `consul lock` defaults to 3. This
  will cause the lock monitor to retry up to 3 times, waiting 1s between
  each attempt if it gets a 500 error from the Consul servers. For the
  vast majority of use cases this is desirable to prevent the lock from
  being given up during a brief period of Consul unavailability. If you
  want to get the previous default behavior you will need to set the
  `-monitor-retry=0` option.

IMPROVEMENTS:

* Consul is now built with Go 1.5.2
* Added source IP address and port information to RPC-related log error
  messages and HTTP access logs [[GH-1513](https://github.com/hashicorp/consul/issues/1513)] [[GH-1448](https://github.com/hashicorp/consul/issues/1448)]
* API clients configured for insecure SSL now use an HTTP transport that's
  set up the same way as the Go default transport [[GH-1526](https://github.com/hashicorp/consul/issues/1526)]
* Added new per-host telemetry on DNS requests [[GH-1537](https://github.com/hashicorp/consul/issues/1537)]
* Added support for reaping child processes which is useful when running
  Consul as PID 1 in Docker containers [[GH-1539](https://github.com/hashicorp/consul/issues/1539)]
* Added new `-ui` command line and `ui` config option that enables a built-in
  Consul web UI, making deployment much simpler [[GH-1543](https://github.com/hashicorp/consul/issues/1543)]
* Added new `-dev` command line option that creates a completely in-memory
  standalone Consul server for development
* Added a Solaris build, now that dependencies have been updated to support
  it [[GH-1568](https://github.com/hashicorp/consul/issues/1568)]
* Added new `-try` option to `consul lock` to allow it to timeout with an error
  if it doesn't acquire the lock [[GH-1567](https://github.com/hashicorp/consul/issues/1567)]
* Added a new `-monitor-retry` option to `consul lock` to help ride out brief
  periods of Consul unavailabily without causing the lock to be given up [[GH-1567](https://github.com/hashicorp/consul/issues/1567)]

BUG FIXES:

* Fixed broken settings icon in web UI [[GH-1469](https://github.com/hashicorp/consul/issues/1469)]
* Fixed a web UI bug where the supplied token wasn't being passed into
  the internal endpoint, breaking some pages when multiple datacenters
  were present [[GH-1071](https://github.com/hashicorp/consul/issues/1071)]

## 0.6.0 (December 3, 2015)

BACKWARDS INCOMPATIBILITIES:

* A KV lock acquisition operation will now allow the lock holder to
  update the key's contents without giving up the lock by doing another
  PUT with `?acquire=<session>` and providing the same session that
  is holding the lock. Previously, this operation would fail.

FEATURES:

* Service ACLs now apply to service discovery [[GH-1024](https://github.com/hashicorp/consul/issues/1024)]
* Added event ACLs to guard firing user events [[GH-1046](https://github.com/hashicorp/consul/issues/1046)]
* Added keyring ACLs for gossip encryption keyring operations [[GH-1090](https://github.com/hashicorp/consul/issues/1090)]
* Added a new TCP check type that does a connect as a check [[GH-1130](https://github.com/hashicorp/consul/issues/1130)]
* Added new "tag override" feature that lets catalog updates to a
  service's tags flow down to agents [[GH-1187](https://github.com/hashicorp/consul/issues/1187)]
* Ported in-memory database from LMDB to an immutable radix tree to improve
  read throughput, reduce garbage collection pressure, and make Consul 100%
  pure Go [[GH-1291](https://github.com/hashicorp/consul/issues/1291)]
* Added support for sending telemetry to DogStatsD [[GH-1293](https://github.com/hashicorp/consul/issues/1293)]
* Added new network tomography subsystem that estimates the network
  round trip times between nodes and exposes that in raw APIs, as well
  as in existing APIs (find the service node nearest node X); also
  includes a new `consul rtt` command to query interactively [[GH-1331](https://github.com/hashicorp/consul/issues/1331)]
* Consul now builds under Go 1.5.1 by default [[GH-1345](https://github.com/hashicorp/consul/issues/1345)]
* Added built-in support for running health checks inside Docker containers
  [[GH-1343](https://github.com/hashicorp/consul/issues/1343)]
* Added prepared queries which support service health queries with rich
  features such as filters for multiple tags and failover to remote datacenters
  based on network coordinates; these are available via HTTP as well as the
  DNS interface [[GH-1389](https://github.com/hashicorp/consul/issues/1389)]

BUG FIXES:

* Fixed expired certificates in unit tests [[GH-979](https://github.com/hashicorp/consul/issues/979)]
* Allow services with `/` characters in the UI [[GH-988](https://github.com/hashicorp/consul/issues/988)]
* Added SOA/NXDOMAIN records to negative DNS responses per RFC2308 [[GH-995](https://github.com/hashicorp/consul/issues/995)]
  [[GH-1142](https://github.com/hashicorp/consul/issues/1142)] [[GH-1195](https://github.com/hashicorp/consul/issues/1195)] [[GH-1217](https://github.com/hashicorp/consul/issues/1217)]
* Token hiding in HTTP logs bug fixed [[GH-1020](https://github.com/hashicorp/consul/issues/1020)]
* RFC6598 addresses are accepted as private IPs [[GH-1050](https://github.com/hashicorp/consul/issues/1050)]
* Fixed reverse DNS lookups to recursor [[GH-1137](https://github.com/hashicorp/consul/issues/1137)]
* Removes the trailing `/` added by the `consul lock` command [[GH-1145](https://github.com/hashicorp/consul/issues/1145)]
* Fixed bad lock handler execution during shutdown [[GH-1080](https://github.com/hashicorp/consul/issues/1080)] [[GH-1158](https://github.com/hashicorp/consul/issues/1158)] [[GH-1214](https://github.com/hashicorp/consul/issues/1214)]
* Added missing support for AAAA queries for nodes [[GH-1222](https://github.com/hashicorp/consul/issues/1222)]
* Tokens passed from the CLI or API work for maint mode [[GH-1230](https://github.com/hashicorp/consul/issues/1230)]
* Fixed service deregister/reregister flaps that could happen during
  `consul reload` [[GH-1235](https://github.com/hashicorp/consul/issues/1235)]
* Fixed the Go API client to properly distinguish between expired sessions
  and sessions that don't exist [[GH-1041](https://github.com/hashicorp/consul/issues/1041)]
* Fixed the KV section of the UI to work on Safari [[GH-1321](https://github.com/hashicorp/consul/issues/1321)]
* Cleaned up JavaScript for built-in UI with bug fixes [[GH-1338](https://github.com/hashicorp/consul/issues/1338)]

IMPROVEMENTS:

* Added sorting of `consul members` command output [[GH-969](https://github.com/hashicorp/consul/issues/969)]
* Updated AWS templates for RHEL6, CentOS6 [[GH-992](https://github.com/hashicorp/consul/issues/992)] [[GH-1002](https://github.com/hashicorp/consul/issues/1002)]
* Advertised gossip/rpc addresses can now be configured [[GH-1004](https://github.com/hashicorp/consul/issues/1004)]
* Failed lock acquisition handling now responds based on type of failure
  [[GH-1006](https://github.com/hashicorp/consul/issues/1006)]
* Agents now remember check state across restarts [[GH-1009](https://github.com/hashicorp/consul/issues/1009)]
* Always run ACL tests by default in API tests [[GH-1030](https://github.com/hashicorp/consul/issues/1030)]
* Consul now refuses to start if there are multiple private IPs [[GH-1099](https://github.com/hashicorp/consul/issues/1099)]
* Improved efficiency of servers managing incoming connections from agents
  [[GH-1170](https://github.com/hashicorp/consul/issues/1170)]
* Added logging of the DNS client addresses in error messages [[GH-1166](https://github.com/hashicorp/consul/issues/1166)]
* Added `-http-port` option to change the HTTP API port number [[GH-1167](https://github.com/hashicorp/consul/issues/1167)]
* Atlas integration options are reload-able via SIGHUP [[GH-1199](https://github.com/hashicorp/consul/issues/1199)]
* Atlas endpoint is a configurable option and CLI arg [[GH-1201](https://github.com/hashicorp/consul/issues/1201)]
* Added `-pass-stdin` option to `consul lock` command [[GH-1200](https://github.com/hashicorp/consul/issues/1200)]
* Enables the `/v1/internal/ui/*` endpoints, even if `-ui-dir` isn't set
  [[GH-1215](https://github.com/hashicorp/consul/issues/1215)]
* Added HTTP method to Consul's log output for better debugging [[GH-1270](https://github.com/hashicorp/consul/issues/1270)]
* Lock holders can `?acquire=<session>` a key again with the same session
  that holds the lock to update a key's contents without releasing the
  lock [[GH-1291](https://github.com/hashicorp/consul/issues/1291)]
* Improved an O(n^2) algorithm in the agent's catalog sync code [[GH-1296](https://github.com/hashicorp/consul/issues/1296)]
* Switched to net-rpc-msgpackrpc to reduce RPC overhead [[GH-1307](https://github.com/hashicorp/consul/issues/1307)]
* Removed all uses of the http package's default client and transport in
  Consul to avoid conflicts with other packages [[GH-1310](https://github.com/hashicorp/consul/issues/1310)] [[GH-1327](https://github.com/hashicorp/consul/issues/1327)]
* Added new `X-Consul-Token` HTTP header option to avoid passing tokens
  in the query string [[GH-1318](https://github.com/hashicorp/consul/issues/1318)]
* Increased session TTL max to 24 hours (use with caution, see note added
  to the Session HTTP endpoint documentation) [[GH-1412](https://github.com/hashicorp/consul/issues/1412)]
* Added support to the API client for retrying lock monitoring when Consul
  is unavailable, helping prevent false indications of lost locks (eg. apps
  like Vault can avoid failing over when a Consul leader election occurs)
  [[GH-1457](https://github.com/hashicorp/consul/issues/1457)]
* Added reap of receive buffer space for idle streams in the connection
  pool [[GH-1452](https://github.com/hashicorp/consul/issues/1452)]

MISC:

* Lots of docs fixes
* Lots of Vagrantfile cleanup
* Data migrator utility removed to eliminate cgo dependency [[GH-1309](https://github.com/hashicorp/consul/issues/1309)]

UPGRADE NOTES:

* Consul will refuse to start if the data directory contains an "mdb" folder.
  This folder was used in versions of Consul up to 0.5.1. Consul version 0.5.2
  included a baked-in utility to automatically upgrade the data format, but
  this has been removed in Consul 0.6 to eliminate the dependency on cgo.
* New service read, event firing, and keyring ACLs may require special steps to
  perform during an upgrade if ACLs are enabled and set to deny by default.
* Consul will refuse to start if there are multiple private IPs available, so
  if this is the case you will need to configure Consul's advertise or bind
  addresses before upgrading.

See https://www.consul.io/docs/upgrade-specific.html for detailed upgrade
instructions.

## 0.5.2 (May 18, 2015)

FEATURES:

* Include datacenter in the `members` output
* HTTP Health Check sets user agent "Consul Health Check" [[GH-951](https://github.com/hashicorp/consul/issues/951)]

BUG FIXES:

* Fixed memory leak caused by blocking query [[GH-939](https://github.com/hashicorp/consul/issues/939)]

MISC:

* Remove unused constant [[GH-941](https://github.com/hashicorp/consul/issues/941)]

## 0.5.1 (May 13, 2015)

FEATURES:

 * Ability to configure minimum session TTL. [[GH-821](https://github.com/hashicorp/consul/issues/821)]
 * Ability to set the initial state of a health check when registering [[GH-859](https://github.com/hashicorp/consul/issues/859)]
 * New `configtest` sub-command to verify config validity [[GH-904](https://github.com/hashicorp/consul/issues/904)]
 * ACL enforcement is prefix based for service names [[GH-905](https://github.com/hashicorp/consul/issues/905)]
 * ACLs support upsert for simpler restore and external generation [[GH-909](https://github.com/hashicorp/consul/issues/909)]
 * ACL tokens can be provided per-service during registration [[GH-891](https://github.com/hashicorp/consul/issues/891)]
 * Support for distinct LAN and WAN advertise addresses [[GH-816](https://github.com/hashicorp/consul/issues/816)]
 * Migrating Raft log from LMDB to BoltDB [[GH-857](https://github.com/hashicorp/consul/issues/857)]
 * `session_ttl_min` is now configurable to reduce the minimum TTL [[GH-821](https://github.com/hashicorp/consul/issues/821)]
 * Adding `verify_server_hostname` to protect against server forging [[GH-927](https://github.com/hashicorp/consul/issues/927)]

BUG FIXES:

 * Datacenter is lowercased, fixes DNS lookups [[GH-761](https://github.com/hashicorp/consul/issues/761)]
 * Deregister all checks when service is deregistered [[GH-918](https://github.com/hashicorp/consul/issues/918)]
 * Fixing issues with updates of persisted services [[GH-910](https://github.com/hashicorp/consul/issues/910)]
 * Chained CNAME resolution fixes [[GH-862](https://github.com/hashicorp/consul/issues/862)]
 * Tokens are filtered out of log messages [[GH-860](https://github.com/hashicorp/consul/issues/860)]
 * Fixing anti-entropy issue if servers rollback Raft log [[GH-850](https://github.com/hashicorp/consul/issues/850)]
 * Datacenter name is case insensitive for DNS lookups
 * Queries for invalid datacenters do not leak sockets [[GH-807](https://github.com/hashicorp/consul/issues/807)]

IMPROVEMENTS:

 * HTTP health checks more reliable, avoid KeepAlives [[GH-824](https://github.com/hashicorp/consul/issues/824)]
 * Improved protection against a passive cluster merge
 * SIGTERM is properly handled for graceful shutdown [[GH-827](https://github.com/hashicorp/consul/issues/827)]
 * Better staggering of deferred updates to checks [[GH-884](https://github.com/hashicorp/consul/issues/884)]
 * Configurable stats prefix [[GH-902](https://github.com/hashicorp/consul/issues/902)]
 * Raft uses BoltDB as the backend store. [[GH-857](https://github.com/hashicorp/consul/issues/857)]
 * API RenewPeriodic more resilient to transient errors [[GH-912](https://github.com/hashicorp/consul/issues/912)]

## 0.5.0 (February 19, 2015)

FEATURES:

 * Key rotation support for gossip layer. This allows the `encrypt` key
   to be changed globally.  See "keyring" command. [[GH-336](https://github.com/hashicorp/consul/issues/336)]
 * Options to join the WAN pool on start (`start_join_wan`, `retry_join_wan`) [[GH-477](https://github.com/hashicorp/consul/issues/477)]
 * Optional HTTPS interface [[GH-478](https://github.com/hashicorp/consul/issues/478)]
 * Ephemeral keys via "delete" session behavior. This allows keys to be deleted when
   a session is invalidated instead of having the lock released. Adds new "Behavior"
   field to Session which is configurable. [[GH-487](https://github.com/hashicorp/consul/issues/487)]
 * Reverse DNS lookups via PTR for IPv4 and IPv6 [[GH-475](https://github.com/hashicorp/consul/issues/475)]
 * API added checks and services are persisted. This means services and
   checks will survive a crash or restart. [[GH-497](https://github.com/hashicorp/consul/issues/497)]
 * ACLs can now protect service registration. Users in blacklist mode should
   allow registrations before upgrading to prevent a service disruption. [[GH-506](https://github.com/hashicorp/consul/issues/506)] [[GH-465](https://github.com/hashicorp/consul/issues/465)]
 * Sessions support a heartbeat failure detector via use of TTLs. This adds a new
   "TTL" field to Sessions and a `/v1/session/renew` endpoint. Heartbeats act like a
   failure detector (health check), but are managed by the servers. [[GH-524](https://github.com/hashicorp/consul/issues/524)] [[GH-172](https://github.com/hashicorp/consul/issues/172)]
 * Support for service specific IP addresses. This allows the service to advertise an
   address that is different from the agent. [[GH-229](https://github.com/hashicorp/consul/issues/229)] [[GH-570](https://github.com/hashicorp/consul/issues/570)]
 * Support KV Delete with Check-And-Set  [[GH-589](https://github.com/hashicorp/consul/issues/589)]
 * Merge `armon/consul-api` into `api` as official Go client.
 * Support for distributed locks and semaphores in API client [[GH-594](https://github.com/hashicorp/consul/issues/594)] [[GH-600](https://github.com/hashicorp/consul/issues/600)]
 * Support for native HTTP health checks [[GH-592](https://github.com/hashicorp/consul/issues/592)]
 * Support for node and service maintenance modes [[GH-606](https://github.com/hashicorp/consul/issues/606)]
 * Added new "consul maint" command to easily toggle maintenance modes [[GH-625](https://github.com/hashicorp/consul/issues/625)]
 * Added new "consul lock" command for simple highly-available deployments.
   This lets Consul manage the leader election and easily handle N+1 deployments
   without the applications being Consul aware. [[GH-619](https://github.com/hashicorp/consul/issues/619)]
 * Multiple checks can be associated with a service [[GH-591](https://github.com/hashicorp/consul/issues/591)] [[GH-230](https://github.com/hashicorp/consul/issues/230)]

BUG FIXES:

 * Fixed X-Consul-Index calculation for KV ListKeys
 * Fixed errors under extremely high read parallelism
 * Fixed issue causing event watches to not fire reliably [[GH-479](https://github.com/hashicorp/consul/issues/479)]
 * Fixed non-monotonic X-Consul-Index with key deletion [[GH-577](https://github.com/hashicorp/consul/issues/577)] [[GH-195](https://github.com/hashicorp/consul/issues/195)]
 * Fixed use of default instead of custom TLD in some DNS responses [[GH-582](https://github.com/hashicorp/consul/issues/582)]
 * Fixed memory leaks in API client when an error response is returned [[GH-608](https://github.com/hashicorp/consul/issues/608)]
 * Fixed issues with graceful leave in single-node bootstrap cluster [[GH-621](https://github.com/hashicorp/consul/issues/621)]
 * Fixed issue preventing node reaping [[GH-371](https://github.com/hashicorp/consul/issues/371)]
 * Fixed gossip stability at very large scale
 * Fixed string of rpc error: rpc error: ... no known leader. [[GH-611](https://github.com/hashicorp/consul/issues/611)]
 * Fixed panic in `exec` during cancellation
 * Fixed health check state reset caused by SIGHUP [[GH-693](https://github.com/hashicorp/consul/issues/693)]
 * Fixed bug in UI when multiple datacenters exist.

IMPROVEMENTS:

 * Support "consul exec" in foreign datacenter [[GH-584](https://github.com/hashicorp/consul/issues/584)]
 * Improved K/V blocking query performance [[GH-578](https://github.com/hashicorp/consul/issues/578)]
 * CLI respects CONSUL_RPC_ADDR environment variable to load parameter [[GH-542](https://github.com/hashicorp/consul/issues/542)]
 * Added support for multiple DNS recursors [[GH-448](https://github.com/hashicorp/consul/issues/448)]
 * Added support for defining multiple services per configuration file [[GH-433](https://github.com/hashicorp/consul/issues/433)]
 * Added support for defining multiple checks per configuration file [[GH-433](https://github.com/hashicorp/consul/issues/433)]
 * Allow mixing of service and check definitions in a configuration file [[GH-433](https://github.com/hashicorp/consul/issues/433)]
 * Allow notes for checks in service definition file [[GH-449](https://github.com/hashicorp/consul/issues/449)]
 * Random stagger for agent checks to prevent thundering herd [[GH-546](https://github.com/hashicorp/consul/issues/546)]
 * More useful metrics are sent to statsd/statsite
 * Added configuration to set custom HTTP headers (CORS) [[GH-558](https://github.com/hashicorp/consul/issues/558)]
 * Reject invalid configurations to simplify validation [[GH-576](https://github.com/hashicorp/consul/issues/576)]
 * Guard against accidental cluster mixing [[GH-580](https://github.com/hashicorp/consul/issues/580)] [[GH-260](https://github.com/hashicorp/consul/issues/260)]
 * Added option to filter DNS results on warning [[GH-595](https://github.com/hashicorp/consul/issues/595)]
 * Improve write throughput with raft log caching [[GH-604](https://github.com/hashicorp/consul/issues/604)]
 * Added ability to bind RPC and HTTP listeners to UNIX sockets [[GH-587](https://github.com/hashicorp/consul/issues/587)] [[GH-612](https://github.com/hashicorp/consul/issues/612)]
 * K/V HTTP endpoint returns 400 on conflicting flags [[GH-634](https://github.com/hashicorp/consul/issues/634)] [[GH-432](https://github.com/hashicorp/consul/issues/432)]

MISC:

 * UI confirms before deleting key sub-tree [[GH-520](https://github.com/hashicorp/consul/issues/520)]
 * More useful output in "consul version" [[GH-480](https://github.com/hashicorp/consul/issues/480)]
 * Many documentation improvements
 * Reduce log messages when quorum member is logs [[GH-566](https://github.com/hashicorp/consul/issues/566)]

UPGRADE NOTES:

 * If `acl_default_policy` is "deny", ensure tokens are updated to enable
   service registration to avoid a service disruption. The new ACL policy
   can be submitted with 0.4 before upgrading to 0.5 where it will be
   enforced.

 * Servers running 0.5.X cannot be mixed with older servers. (Any client
   version is fine). There is a 15 minute upgrade window where mixed
   versions are allowed before older servers will panic due to an unsupported
   internal command. This is due to the new KV tombstones which are internal
   to servers.

## 0.4.1 (October 20, 2014)

FEATURES:

 * Adding flags for `-retry-join` to attempt a join with
   configurable retry behavior. [[GH-395](https://github.com/hashicorp/consul/issues/395)]

BUG FIXES:

 * Fixed ACL token in UI
 * Fixed ACL reloading in UI [[GH-323](https://github.com/hashicorp/consul/issues/323)]
 * Fixed long session names in UI [[GH-353](https://github.com/hashicorp/consul/issues/353)]
 * Fixed exit code from remote exec [[GH-346](https://github.com/hashicorp/consul/issues/346)]
 * Fixing only a single watch being run by an agent [[GH-337](https://github.com/hashicorp/consul/issues/337)]
 * Fixing potential race in connection multiplexing
 * Fixing issue with Session ID and ACL ID generation. [[GH-391](https://github.com/hashicorp/consul/issues/391)]
 * Fixing multiple headers for /v1/event/list endpoint [[GH-361](https://github.com/hashicorp/consul/issues/361)]
 * Fixing graceful leave of leader causing invalid Raft peers [[GH-360](https://github.com/hashicorp/consul/issues/360)]
 * Fixing bug with closing TLS connection on error
 * Fixing issue with node reaping [[GH-371](https://github.com/hashicorp/consul/issues/371)]
 * Fixing aggressive deadlock time [[GH-389](https://github.com/hashicorp/consul/issues/389)]
 * Fixing syslog filter level [[GH-272](https://github.com/hashicorp/consul/issues/272)]
 * Serf snapshot compaction works on Windows [[GH-332](https://github.com/hashicorp/consul/issues/332)]
 * Raft snapshots work on Windows [[GH-265](https://github.com/hashicorp/consul/issues/265)]
 * Consul service entry clean by clients now possible
 * Fixing improper deserialization

IMPROVEMENTS:

 * Use "critical" health state instead of "unknown" [[GH-341](https://github.com/hashicorp/consul/issues/341)]
 * Consul service can be targeted for exec [[GH-344](https://github.com/hashicorp/consul/issues/344)]
 * Provide debug logging for session invalidation [[GH-390](https://github.com/hashicorp/consul/issues/390)]
 * Added "Deregister" button to UI [[GH-364](https://github.com/hashicorp/consul/issues/364)]
 * Added `enable_truncate` DNS configuration flag [[GH-376](https://github.com/hashicorp/consul/issues/376)]
 * Reduce mmap() size on 32bit systems [[GH-265](https://github.com/hashicorp/consul/issues/265)]
 * Temporary state is cleaned after an abort [[GH-338](https://github.com/hashicorp/consul/issues/338)] [[GH-178](https://github.com/hashicorp/consul/issues/178)]

MISC:

 * Health state "unknown" being deprecated

## 0.4.0 (September 5, 2014)

FEATURES:

 * Fine-grained ACL system to restrict access to KV store. Clients
   use tokens which can be restricted to (read, write, deny) permissions
   using longest-prefix matches.

 * Watch mechanisms added to invoke a handler when data changes in consul.
   Used with the `consul watch` command, or by specifying `watches` in
   an agent configuration.

 * Event system added to support custom user events. Events are fired using
   the `consul event` command. They are handled using a standard watch.

 * Remote execution using `consul exec`. This allows for command execution on remote
   instances mediated through Consul.

 * RFC-2782 style DNS lookups supported

 * UI improvements, including support for ACLs.

IMPROVEMENTS:

  * DNS case-insensitivity [[GH-189](https://github.com/hashicorp/consul/issues/189)]
  * Support for HTTP `?pretty` parameter to pretty format JSON output.
  * Use $SHELL when invoking handlers. [[GH-237](https://github.com/hashicorp/consul/issues/237)]
  * Agent takes the `-encrypt` CLI Flag [[GH-245](https://github.com/hashicorp/consul/issues/245)]
  * New `statsd_add` config for Statsd support. [[GH-247](https://github.com/hashicorp/consul/issues/247)]
  * New `addresses` config for providing an override to `client_addr` for
    DNS, HTTP, or RPC endpoints. [[GH-301](https://github.com/hashicorp/consul/issues/301)] [[GH-253](https://github.com/hashicorp/consul/issues/253)]
  * Support [Checkpoint](http://checkpoint.hashicorp.com) for security bulletins
    and update announcements.

BUG FIXES:

  * Fixed race condition in `-bootstrap-expect` [[GH-254](https://github.com/hashicorp/consul/issues/254)]
  * Require PUT to /v1/session/destroy [[GH-285](https://github.com/hashicorp/consul/issues/285)]
  * Fixed registration race condition [[GH-300](https://github.com/hashicorp/consul/issues/300)] [[GH-279](https://github.com/hashicorp/consul/issues/279)]

UPGRADE NOTES:

  * ACL support should not be enabled until all server nodes are running
  Consul 0.4. Mixed server versions with ACL support enabled may result in
  panics.

## 0.3.1 (July 21, 2014)

FEATURES:

  * Improved bootstrapping process, thanks to @robxu9

BUG FIXES:

  * Fixed issue with service re-registration [[GH-216](https://github.com/hashicorp/consul/issues/216)]
  * Fixed handling of `-rejoin` flag
  * Restored 0.2 TLS behavior, thanks to @nelhage [[GH-233](https://github.com/hashicorp/consul/issues/233)]
  * Fix the statsite flags, thanks to @nelhage [[GH-243](https://github.com/hashicorp/consul/issues/243)]
  * Fixed filters on critical / non-passing checks [[GH-241](https://github.com/hashicorp/consul/issues/241)]
  * Fixed initial log compaction crash [[GH-297](https://github.com/hashicorp/consul/issues/297)]

IMPROVEMENTS:

  * UI Improvements
  * Improved handling of Serf snapshot data
  * Increase reliability of failure detector
  * More useful logging messages


## 0.3.0 (June 13, 2014)

FEATURES:

  * Better, faster, cleaner UI [[GH-194](https://github.com/hashicorp/consul/issues/194)] [[GH-196](https://github.com/hashicorp/consul/issues/196)]
  * Sessions, which  act as a binding layer between
  nodes, checks and KV data. [[GH-162](https://github.com/hashicorp/consul/issues/162)]
  * Key locking. KV data integrates with sessions to
  enable distributed locking. [[GH-162](https://github.com/hashicorp/consul/issues/162)]
  * DNS lookups can do stale reads and TTLs. [[GH-200](https://github.com/hashicorp/consul/issues/200)]
  * Added new /v1/agent/self endpoint [[GH-173](https://github.com/hashicorp/consul/issues/173)]
  * `reload` command can be used to trigger configuration
  reload from the CLI [[GH-142](https://github.com/hashicorp/consul/issues/142)]

IMPROVEMENTS:

  * `members` has a much cleaner output format [[GH-143](https://github.com/hashicorp/consul/issues/143)]
  * `info` includes build version information
  * Sorted results for datacneter list [[GH-198](https://github.com/hashicorp/consul/issues/198)]
  * Switch multiplexing to yamux
  * Allow multiple CA certs in ca_file [[GH-174](https://github.com/hashicorp/consul/issues/174)]
  * Enable logging to syslog. [[GH-105](https://github.com/hashicorp/consul/issues/105)]
  * Allow raw key value lookup [[GH-150](https://github.com/hashicorp/consul/issues/150)]
  * Log encryption enabled [[GH-151](https://github.com/hashicorp/consul/issues/151)]
  * Support `-rejoin` to rejoin a cluster after a previous leave. [[GH-110](https://github.com/hashicorp/consul/issues/110)]
  * Support the "any" wildcard for v1/health/state/ [[GH-152](https://github.com/hashicorp/consul/issues/152)]
  * Defer sync of health check output [[GH-157](https://github.com/hashicorp/consul/issues/157)]
  * Provide output for serfHealth check [[GH-176](https://github.com/hashicorp/consul/issues/176)]
  * Datacenter name is validated [[GH-169](https://github.com/hashicorp/consul/issues/169)]
  * Configurable syslog facilities [[GH-170](https://github.com/hashicorp/consul/issues/170)]
  * Pipelining replication of writes
  * Raft group commits
  * Increased stability of leader terms
  * Prevent previously left nodes from causing re-elections

BUG FIXES:

  * Fixed memory leak in in-memory stats system
  * Fixing race between RPC and Raft init [[GH-160](https://github.com/hashicorp/consul/issues/160)]
  * Server-local RPC is avoids network [[GH-148](https://github.com/hashicorp/consul/issues/148)]
  * Fixing builds for older OSX [[GH-147](https://github.com/hashicorp/consul/issues/147)]

MISC:

  * Fixed missing prefixes on some log messages
  * Removed the `-role` filter of `members` command
  * Lots of docs fixes

## 0.2.1 (May 20, 2014)

IMPROVEMENTS:

  * Improved the URL formatting for the key/value editor in the Web UI.
      Importantly, the editor now allows editing keys with dashes in the
      name. [[GH-119](https://github.com/hashicorp/consul/issues/119)]
  * The web UI now has cancel and delete folder actions in the key/value
      editor. [[GH-124](https://github.com/hashicorp/consul/issues/124)], [[GH-122](https://github.com/hashicorp/consul/issues/122)]
  * Add flag to agent to write pid to a file. [[GH-106](https://github.com/hashicorp/consul/issues/106)]
  * Time out commands if Raft exceeds command enqueue timeout
  * Adding support for the `-advertise` CLI flag. [[GH-156](https://github.com/hashicorp/consul/issues/156)]
  * Fixing potential name conflicts on the WAN gossip ring [[GH-158](https://github.com/hashicorp/consul/issues/158)]
  * /v1/catalog/services returns an empty slice instead of null. [[GH-145](https://github.com/hashicorp/consul/issues/145)]
  * `members` command returns exit code 2 if no results. [[GH-116](https://github.com/hashicorp/consul/issues/116)]

BUG FIXES:

  * Renaming "separator" to "separator". This is the correct spelling,
      but both spellings are respected for backwards compatibility. [[GH-101](https://github.com/hashicorp/consul/issues/101)]
  * Private IP is properly found on Windows clients.
  * Windows agents won't show "failed to decode" errors on every RPC
      request.
  * Fixed memory leak with RPC clients. [[GH-149](https://github.com/hashicorp/consul/issues/149)]
  * Serf name conflict resolution disabled. [[GH-97](https://github.com/hashicorp/consul/issues/97)]
  * Raft deadlock possibility fixed. [[GH-141](https://github.com/hashicorp/consul/issues/141)]

MISC:

  * Updating to latest version of LMDB
  * Reduced the limit of KV entries to 512KB. [[GH-123](https://github.com/hashicorp/consul/issues/123)].
  * Warn if any Raft log exceeds 1MB
  * Lots of docs fixes

## 0.2.0 (May 1, 2014)

FEATURES:

  * Adding Web UI for Consul. This is enabled by providing the `-ui-dir` flag
      with the path to the web directory. The UI is visited at the standard HTTP
      address (Defaults to http://127.0.0.1:8500/). There is a demo
  [available here](http://demo.consul.io).
  * Adding new read consistency modes. `?consistent` can be used for strongly
      consistent reads without caveats. `?stale` can be used for stale reads to
      allow for higher throughput and read scalability. [[GH-68](https://github.com/hashicorp/consul/issues/68)]
  * /v1/health/service/ endpoint can take an optional `?passing` flag
      to filter to only nodes with passing results. [[GH-57](https://github.com/hashicorp/consul/issues/57)]
  * The KV endpoint supports listing keys with the `?keys` query parameter,
      and limited up to a separator using `?separator=`.

IMPROVEMENTS:

  * Health check output goes into separate `Output` field instead
      of overriding `Notes`. [[GH-59](https://github.com/hashicorp/consul/issues/59)]
  * Adding a minimum check interval to prevent checks with extremely
      low intervals fork bombing. [[GH-64](https://github.com/hashicorp/consul/issues/64)]
  * Raft peer set cleared on leave. [[GH-69](https://github.com/hashicorp/consul/issues/69)]
  * Case insensitive parsing checks. [[GH-78](https://github.com/hashicorp/consul/issues/78)]
  * Increase limit of DB size and Raft log on 64bit systems. [[GH-81](https://github.com/hashicorp/consul/issues/81)]
  * Output of health checks limited to 4K. [[GH-83](https://github.com/hashicorp/consul/issues/83)]
  * More warnings if GOMAXPROCS == 1 [[GH-87](https://github.com/hashicorp/consul/issues/87)]
  * Added runtime information to `consul info`

BUG FIXES:

  * Fixed 404 on /v1/agent/service/deregister and
      /v1/agent/check/deregister. [[GH-95](https://github.com/hashicorp/consul/issues/95)]
  * Fixed JSON parsing for /v1/agent/check/register [[GH-60](https://github.com/hashicorp/consul/issues/60)]
  * DNS parser can handler period in a tag name. [[GH-39](https://github.com/hashicorp/consul/issues/39)]
  * "application/json" content-type is sent on HTTP requests. [[GH-45](https://github.com/hashicorp/consul/issues/45)]
  * Work around for LMDB delete issue. [[GH-85](https://github.com/hashicorp/consul/issues/85)]
  * Fixed tag gossip propagation for rapid restart. [[GH-86](https://github.com/hashicorp/consul/issues/86)]

MISC:

  * More conservative timing values for Raft
  * Provide a warning if attempting to commit a very large Raft entry
  * Improved timeliness of registration when server is in bootstrap mode. [[GH-72](https://github.com/hashicorp/consul/issues/72)]

## 0.1.0 (April 17, 2014)

  * Initial release
