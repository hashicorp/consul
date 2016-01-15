## 0.7.0 (UNRELEASED)

IMPROVEMENTS:

BUG FIXES:

## 0.6.3 (January 15, 2016)

BUG FIXES:

* Fixed an issue when running Consul as PID 1 in a Docker container where
  it could consume CPU and show spurious failures for health checks, watch
  handlers, and `consul exec` commands [GH-1592]

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
  messages and HTTP access logs [GH-1513] [GH-1448]
* API clients configured for insecure SSL now use an HTTP transport that's
  set up the same way as the Go default transport [GH-1526]
* Added new per-host telemery on DNS requests [GH-1537]
* Added support for reaping child processes which is useful when running
  Consul as PID 1 in Docker containers [GH-1539]
* Added new `-ui` command line and `ui` config option that enables a built-in
  Consul web UI, making deployment much simpler [GH-1543]
* Added new `-dev` command line option that creates a completely in-memory
  standalone Consul server for development
* Added a Solaris build, now that dependencies have been updated to support
  it [GH-1568]
* Added new `-try` option to `consul lock` to allow it to timeout with an error
  if it doesn't acquire the lock [GH-1567]
* Added a new `-monitor-retry` option to `consul lock` to help ride out brief
  periods of Consul unavailabily without causing the lock to be given up [GH-1567]

BUG FIXES:

* Fixed broken settings icon in web UI [GH-1469]
* Fixed a web UI bug where the supplied token wasn't being passed into
  the internal endpoint, breaking some pages when multiple datacenters
  were present [GH-1071]

## 0.6.0 (December 3, 2015)

BACKWARDS INCOMPATIBILITIES:

* A KV lock acquisition operation will now allow the lock holder to
  update the key's contents without giving up the lock by doing another
  PUT with `?acquire=<session>` and providing the same session that
  is holding the lock. Previously, this operation would fail.

FEATURES:

* Service ACLs now apply to service discovery [GH-1024]
* Added event ACLs to guard firing user events [GH-1046]
* Added keyring ACLs for gossip encryption keyring operations [GH-1090]
* Added a new TCP check type that does a connect as a check [GH-1130]
* Added new "tag override" feature that lets catalog updates to a
  service's tags flow down to agents [GH-1187]
* Ported in-memory database from LMDB to an immutable radix tree to improve
  read throughput, reduce garbage collection pressure, and make Consul 100%
  pure Go [GH-1291]
* Added support for sending telemetry to DogStatsD [GH-1293]
* Added new network tomography subsystem that estimates the network
  round trip times between nodes and exposes that in raw APIs, as well
  as in existing APIs (find the service node nearest node X); also
  includes a new `consul rtt` command to query interactively [GH-1331]
* Consul now builds under Go 1.5.1 by default [GH-1345]
* Added built-in support for running health checks inside Docker containers
  [GH-1343]
* Added prepared queries which support service health queries with rich
  features such as filters for multiple tags and failover to remote datacenters
  based on network coordinates; these are available via HTTP as well as the
  DNS interface [GH-1389]

BUG FIXES:

* Fixed expired certificates in unit tests [GH-979]
* Allow services with `/` characters in the UI [GH-988]
* Added SOA/NXDOMAIN records to negative DNS responses per RFC2308 [GH-995]
  [GH-1142] [GH-1195] [GH-1217]
* Token hiding in HTTP logs bug fixed [GH-1020]
* RFC6598 addresses are accepted as private IPs [GH-1050]
* Fixed reverse DNS lookups to recursor [GH-1137]
* Removes the trailing `/` added by the `consul lock` command [GH-1145]
* Fixed bad lock handler execution during shutdown [GH-1080] [GH-1158] [GH-1214]
* Added missing support for AAAA queries for nodes [GH-1222]
* Tokens passed from the CLI or API work for maint mode [GH-1230]
* Fixed service derigister/reregister flaps that could happen during
  `consul reload` [GH-1235]
* Fixed the Go API client to properly distinguish between expired sessions
  and sessions that don't exist [GH-1041]
* Fixed the KV section of the UI to work on Safari [GH-1321]
* Cleaned up Javascript for built-in UI with bug fixes [GH-1338]

IMPROVEMENTS:

* Added sorting of `consul members` command output [GH-969]
* Updated AWS templates for RHEL6, CentOS6 [GH-992] [GH-1002]
* Advertised gossip/rpc addresses can now be configured [GH-1004]
* Failed lock acquisition handling now responds based on type of failure
  [GH-1006]
* Agents now remember check state across restarts [GH-1009]
* Always run ACL tests by default in API tests [GH-1030]
* Consul now refuses to start if there are multiple private IPs [GH-1099]
* Improved efficiency of servers managing incoming connections from agents
  [GH-1170]
* Added logging of the DNS client addresses in error messages [GH-1166]
* Added `-http-port` option to change the HTTP API port number [GH-1167]
* Atlas integration options are reload-able via SIGHUP [GH-1199]
* Atlas endpoint is a configurable option and CLI arg [GH-1201]
* Added `-pass-stdin` option to `consul lock` command [GH-1200]
* Enables the `/v1/internal/ui/*` endpoints, even if `-ui-dir` isn't set
  [GH-1215]
* Added HTTP method to Consul's log output for better debugging [GH-1270]
* Lock holders can `?acquire=<session>` a key again with the same session
  that holds the lock to update a key's contents without releasing the
  lock [GH-1291]
* Improved an O(n^2) algorithm in the agent's catalog sync code [GH-1296]
* Switched to net-rpc-msgpackrpc to reduce RPC overhead [GH-1307]
* Removed all uses of the http package's default client and transport in
  Consul to avoid conflicts with other packages [GH-1310] [GH-1327]
* Added new `X-Consul-Token` HTTP header option to avoid passing tokens
  in the query string [GH-1318]
* Increased session TTL max to 24 hours (use with caution, see note added
  to the Session HTTP endpoint documentation) [GH-1412]
* Added support to the API client for retrying lock monitoring when Consul
  is unavailable, helping prevent false indications of lost locks (eg. apps
  like Vault can avoid failing over when a Consul leader election occurs)
  [GH-1457]
* Added reap of receive buffer space for idle streams in the connection
  pool [GH-1452]

MISC:

* Lots of docs fixes
* Lots of Vagrantfile cleanup
* Data migrator utility removed to eliminate cgo dependency [GH-1309]

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
* HTTP Health Check sets user agent "Consul Health Check" [GH-951]

BUG FIXES:

* Fixed memory leak caused by blocking query [GH-939]

MISC:

* Remove unused constant [GH-941]

## 0.5.1 (May 13, 2015)

FEATURES:

 * Ability to configure minimum session TTL. [GH-821]
 * Ability to set the initial state of a health check when registering [GH-859]
 * New `configtest` sub-command to verify config validity [GH-904]
 * ACL enforcement is prefix based for service names [GH-905]
 * ACLs support upsert for simpler restore and external generation [GH-909]
 * ACL tokens can be provided per-service during registration [GH-891]
 * Support for distinct LAN and WAN advertise addresses [GH-816]
 * Migrating Raft log from LMDB to BoltDB [GH-857]
 * `session_ttl_min` is now configurable to reduce the minimum TTL [GH-821]
 * Adding `verify_server_hostname` to protect against server forging [GH-927]

BUG FIXES:

 * Datacenter is lowercased, fixes DNS lookups [GH-761]
 * Deregister all checks when service is deregistered [GH-918]
 * Fixing issues with updates of persisted services [GH-910]
 * Chained CNAME resolution fixes [GH-862]
 * Tokens are filtered out of log messages [GH-860]
 * Fixing anti-entropy issue if servers rollback Raft log [GH-850]
 * Datacenter name is case insensitive for DNS lookups
 * Queries for invalid datacenters do not leak sockets [GH-807]

IMPROVEMENTS:

 * HTTP health checks more reliable, avoid KeepAlives [GH-824]
 * Improved protection against a passive cluster merge
 * SIGTERM is properly handled for graceful shutdown [GH-827]
 * Better staggering of deferred updates to checks [GH-884]
 * Configurable stats prefix [GH-902]
 * Raft uses BoltDB as the backend store. [GH-857]
 * API RenewPeriodic more resilient to transient errors [GH-912]

## 0.5.0 (February 19, 2015)

FEATURES:

 * Key rotation support for gossip layer. This allows the `encrypt` key
   to be changed globally.  See "keyring" command. [GH-336]
 * Options to join the WAN pool on start (`start_join_wan`, `retry_join_wan`) [GH-477]
 * Optional HTTPS interface [GH-478]
 * Ephemeral keys via "delete" session behavior. This allows keys to be deleted when
   a session is invalidated instead of having the lock released. Adds new "Behavior"
   field to Session which is configurable. [GH-487]
 * Reverse DNS lookups via PTR for IPv4 and IPv6 [GH-475]
 * API added checks and services are persisted. This means services and
   checks will survive a crash or restart. [GH-497]
 * ACLs can now protect service registration. Users in blacklist mode should
   allow registrations before upgrading to prevent a service disruption. [GH-506] [GH-465]
 * Sessions support a heartbeat failure detector via use of TTLs. This adds a new
   "TTL" field to Sessions and a `/v1/session/renew` endpoint. Heartbeats act like a
   failure detector (health check), but are managed by the servers. [GH-524] [GH-172]
 * Support for service specific IP addresses. This allows the service to advertise an
   address that is different from the agent. [GH-229] [GH-570]
 * Support KV Delete with Check-And-Set  [GH-589]
 * Merge `armon/consul-api` into `api` as official Go client.
 * Support for distributed locks and semaphores in API client [GH-594] [GH-600]
 * Support for native HTTP health checks [GH-592]
 * Support for node and service maintanence modes [GH-606]
 * Added new "consul maint" command to easily toggle maintanence modes [GH-625]
 * Added new "consul lock" command for simple highly-available deployments.
   This lets Consul manage the leader election and easily handle N+1 deployments
   without the applications being Consul aware. [GH-619]
 * Multiple checks can be associated with a service [GH-591] [GH-230]

BUG FIXES:

 * Fixed X-Consul-Index calculation for KV ListKeys
 * Fixed errors under extremely high read parallelism
 * Fixed issue causing event watches to not fire reliably [GH-479]
 * Fixed non-monotonic X-Consul-Index with key deletion [GH-577] [GH-195]
 * Fixed use of default instead of custom TLD in some DNS responses [GH-582]
 * Fixed memory leaks in API client when an error response is returned [GH-608]
 * Fixed issues with graceful leave in single-node bootstrap cluster [GH-621]
 * Fixed issue preventing node reaping [GH-371]
 * Fixed gossip stability at very large scale
 * Fixed string of rpc error: rpc error: ... no known leader. [GH-611]
 * Fixed panic in `exec` during cancellation
 * Fixed health check state reset caused by SIGHUP [GH-693]
 * Fixed bug in UI when multiple datacenters exist.

IMPROVEMENTS:

 * Support "consul exec" in foreign datacenter [GH-584]
 * Improved K/V blocking query performance [GH-578]
 * CLI respects CONSUL_RPC_ADDR environment variable to load parameter [GH-542]
 * Added support for multiple DNS recursors [GH-448]
 * Added support for defining multiple services per configuration file [GH-433]
 * Added support for defining multiple checks per configuration file [GH-433]
 * Allow mixing of service and check definitions in a configuration file [GH-433]
 * Allow notes for checks in service definition file [GH-449]
 * Random stagger for agent checks to prevent thundering herd [GH-546]
 * More useful metrics are sent to statsd/statsite
 * Added configuration to set custom HTTP headers (CORS) [GH-558]
 * Reject invalid configurations to simplify validation [GH-576]
 * Guard against accidental cluster mixing [GH-580] [GH-260]
 * Added option to filter DNS results on warning [GH-595]
 * Improve write throughput with raft log caching [GH-604]
 * Added ability to bind RPC and HTTP listeners to UNIX sockets [GH-587] [GH-612]
 * K/V HTTP endpoint returns 400 on conflicting flags [GH-634] [GH-432]

MISC:

 * UI confirms before deleting key sub-tree [GH-520]
 * More useful output in "consul version" [GH-480]
 * Many documentation improvements
 * Reduce log messages when quorum member is logs [GH-566]

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
   configurable retry behavior. [GH-395]

BUG FIXES:

 * Fixed ACL token in UI
 * Fixed ACL reloading in UI [GH-323]
 * Fixed long session names in UI [GH-353]
 * Fixed exit code from remote exec [GH-346]
 * Fixing only a single watch being run by an agent [GH-337]
 * Fixing potential race in connection multiplexing
 * Fixing issue with Session ID and ACL ID generation. [GH-391]
 * Fixing multiple headers for /v1/event/list endpoint [GH-361]
 * Fixing graceful leave of leader causing invalid Raft peers [GH-360]
 * Fixing bug with closing TLS connction on error
 * Fixing issue with node reaping [GH-371]
 * Fixing aggressive deadlock time [GH-389]
 * Fixing syslog filter level [GH-272]
 * Serf snapshot compaction works on Windows [GH-332]
 * Raft snapshots work on Windows [GH-265]
 * Consul service entry clean by clients now possible
 * Fixing improper deserialization

IMPROVEMENTS:

 * Use "critical" health state instead of "unknown" [GH-341]
 * Consul service can be targed for exec [GH-344]
 * Provide debug logging for session invalidation [GH-390]
 * Added "Deregister" button to UI [GH-364]
 * Added `enable_truncate` DNS configuration flag [GH-376]
 * Reduce mmap() size on 32bit systems [GH-265]
 * Temporary state is cleaned after an abort [GH-338] [GH-178]

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

  * DNS case-insensitivity [GH-189]
  * Support for HTTP `?pretty` parameter to pretty format JSON output.
  * Use $SHELL when invoking handlers. [GH-237]
  * Agent takes the `-encrypt` CLI Flag [GH-245]
  * New `statsd_add` config for Statsd support. [GH-247]
  * New `addresses` config for providing an override to `client_addr` for
    DNS, HTTP, or RPC endpoints. [GH-301] [GH-253]
  * Support [Checkpoint](http://checkpoint.hashicorp.com) for security bulletins
    and update announcements.

BUG FIXES:

  * Fixed race condition in `-bootstrap-expect` [GH-254]
  * Require PUT to /v1/session/destroy [GH-285]
  * Fixed registration race condition [GH-300] [GH-279]

UPGRADE NOTES:

  * ACL support should not be enabled until all server nodes are running
  Consul 0.4. Mixed server versions with ACL support enabled may result in
  panics.

## 0.3.1 (July 21, 2014)

FEATURES:

  * Improved bootstrapping process, thanks to @robxu9

BUG FIXES:

  * Fixed issue with service re-registration [GH-216]
  * Fixed handling of `-rejoin` flag
  * Restored 0.2 TLS behavior, thanks to @nelhage [GH-233]
  * Fix the statsite flags, thanks to @nelhage [GH-243]
  * Fixed filters on criticial / non-passing checks [GH-241]
  * Fixed initial log compaction crash [GH-297]

IMPROVEMENTS:

  * UI Improvements
  * Improved handling of Serf snapshot data
  * Increase reliability of failure detector
  * More useful logging messages


## 0.3.0 (June 13, 2014)

FEATURES:

  * Better, faster, cleaner UI [GH-194] [GH-196]
  * Sessions, which  act as a binding layer between
  nodes, checks and KV data. [GH-162]
  * Key locking. KV data integrates with sessions to
  enable distributed locking. [GH-162]
  * DNS lookups can do stale reads and TTLs. [GH-200]
  * Added new /v1/agent/self endpoint [GH-173]
  * `reload` command can be used to trigger configuration
  reload from the CLI [GH-142]

IMPROVEMENTS:

  * `members` has a much cleaner output format [GH-143]
  * `info` includes build version information
  * Sorted results for datacneter list [GH-198]
  * Switch multiplexing to yamux
  * Allow multiple CA certis in ca_file [GH-174]
  * Enable logging to syslog. [GH-105]
  * Allow raw key value lookup [GH-150]
  * Log encryption enabled [GH-151]
  * Support `-rejoin` to rejoin a cluster after a previous leave. [GH-110]
  * Support the "any" wildcard for v1/health/state/ [GH-152]
  * Defer sync of health check output [GH-157]
  * Provide output for serfHealth check [GH-176]
  * Datacenter name is validated [GH-169]
  * Configurable syslog facilities [GH-170]
  * Pipelining replication of writes
  * Raft group commits
  * Increased stability of leader terms
  * Prevent previously left nodes from causing re-elections

BUG FIXES:

  * Fixed memory leak in in-memory stats system
  * Fixing race between RPC and Raft init [GH-160]
  * Server-local RPC is avoids network [GH-148]
  * Fixing builds for older OSX [GH-147]

MISC:

  * Fixed missing prefixes on some log messages
  * Removed the `-role` filter of `members` command
  * Lots of docs fixes

## 0.2.1 (May 20, 2014)

IMPROVEMENTS:

  * Improved the URL formatting for the key/value editor in the Web UI.
      Importantly, the editor now allows editing keys with dashes in the
      name. [GH-119]
  * The web UI now has cancel and delete folder actions in the key/value
      editor. [GH-124], [GH-122]
  * Add flag to agent to write pid to a file. [GH-106]
  * Time out commands if Raft exceeds command enqueue timeout
  * Adding support for the `-advertise` CLI flag. [GH-156]
  * Fixing potential name conflicts on the WAN gossip ring [GH-158]
  * /v1/catalog/services returns an empty slice instead of null. [GH-145]
  * `members` command returns exit code 2 if no results. [GH-116]

BUG FIXES:

  * Renaming "separator" to "separator". This is the correct spelling,
      but both spellings are respected for backwards compatibility. [GH-101]
  * Private IP is properly found on Windows clients.
  * Windows agents won't show "failed to decode" errors on every RPC
      request.
  * Fixed memory leak with RPC clients. [GH-149]
  * Serf name conflict resoultion disabled. [GH-97]
  * Raft deadlock possibility fixed. [GH-141]

MISC:

  * Updating to latest version of LMDB
  * Reduced the limit of KV entries to 512KB. [GH-123].
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
      allow for higher throughput and read scalability. [GH-68]
  * /v1/health/service/ endpoint can take an optional `?passing` flag
      to filter to only nodes with passing results. [GH-57]
  * The KV endpoint suports listing keys with the `?keys` query parameter,
      and limited up to a separator using `?separator=`.

IMPROVEMENTS:

  * Health check output goes into separate `Output` field instead
      of overriding `Notes`. [GH-59]
  * Adding a minimum check interval to prevent checks with extremely
      low intervals fork bombing. [GH-64]
  * Raft peer set cleared on leave. [GH-69]
  * Case insensitive parsing checks. [GH-78]
  * Increase limit of DB size and Raft log on 64bit systems. [GH-81]
  * Output of health checks limited to 4K. [GH-83]
  * More warnings if GOMAXPROCS == 1 [GH-87]
  * Added runtime information to `consul info`

BUG FIXES:

  * Fixed 404 on /v1/agent/service/deregister and
      /v1/agent/check/deregister. [GH-95]
  * Fixed JSON parsing for /v1/agent/check/register [GH-60]
  * DNS parser can handler period in a tag name. [GH-39]
  * "application/json" content-type is sent on HTTP requests. [GH-45]
  * Work around for LMDB delete issue. [GH-85]
  * Fixed tag gossip propagation for rapid restart. [GH-86]

MISC:

  * More conservative timing values for Raft
  * Provide a warning if attempting to commit a very large Raft entry
  * Improved timeliness of registration when server is in bootstrap mode. [GH-72]

## 0.1.0 (April 17, 2014)

  * Initial release

