## 0.2.0 (unreleased)

FEATURES:

  * Adding new read consistency modes. `?consistent` can be used for strongly
  consistent reads without caveats. `?stale` can be used for stale reads to
  allow for higher throughput and read scalability. [GH-68]

  * /v1/health/service/ endpoint can take an optional `?passing` flag
  to filter to only nodes with passing results. [GH-57]

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

