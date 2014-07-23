## 0.3.2 (Unreleased)

IMPROVEMENTS:

  * DNS case-insensitivity [GH-189]

## 0.3.1 (July 21, 2014)

FEATURES:

  * Improved bootstrapping process, thanks to @robxu9

BUG FIXES:

  * Fixed issue with service re-registration [GH-216]
  * Fixed handling of `-rejoin` flag
  * Restored 0.2 TLS behavior, thanks to @nelhage [GH-233]
  * Fix the statsite flags, thanks to @nelhage [GH-243]
  * Fixed filters on criticial / non-passing checks [GH-241]

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

  * Renaming "seperator" to "separator". This is the correct spelling,
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
      and limited up to a seperator using `?seperator=`.

IMPROVEMENTS:

  * Health check output goes into seperate `Output` field instead
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
  * Fixed tag gossip propogation for rapid restart. [GH-86]

MISC:

  * More conservative timing values for Raft
  * Provide a warning if attempting to commit a very large Raft entry
  * Improved timeliness of registration when server is in bootstrap mode. [GH-72]

## 0.1.0 (April 17, 2014)

  * Initial release

