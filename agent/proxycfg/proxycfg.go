// Package proxycfg provides a component that monitors local agent state for
// Connect proxy service registrations and maintains the necessary cache state
// for those proxies locally. Local cache state keeps pull based proxies (e.g.
// the built in one) performant even on first request/startup, and allows for
// push-based proxy APIs (e.g. xDS for Envoy) to be notified of updates to the
// proxy configuration.
//
// The relationship with other agent components looks like this:
//
//     +------------------------------------------+
//     | AGENT                                    |
//     |                                          |
//     | +--------+  1.  +----------+             |
//     | | local  |<-----+ proxycfg |<--------+   |
//     | | state  +----->| Manager  |<---+    |   |
//     | +--------+  2.  +^---+-----+    |    |   |
//     |                5.|   |          |    |   |
//     |       +----------+   |  +-------+--+ |4. |
//     |       |              +->| proxycfg | |   |
//     |       |            3.|  |  State   | |   |
//     |       |              |  +----------+ |   |
//     |       |              |               |   |
//     |       |              |  +----------+ |   |
//     |       |              +->| proxycfg +-+   |
//     |       |                 |  State   |     |
//     |       |                 +----------+     |
//     |       |6.                                |
//     |  +----v---+                              |
//     |  |   xDS  |                              |
//     |  | Server |                              |
//     |  +--------+                              |
//     |                                          |
//     +------------------------------------------+
//
//  1. Manager watches local state for changes.
//  2. On local state change manager is notified and iterates through state
//     looking for proxy service registrations.
//  3. For each proxy service registered, the manager maintains a State
//     instance, recreating on change, removing when deregistered.
//  4. State instance copies the parts of the the proxy service registration
//     needed to configure proxy, and sets up blocking watches on the local
//     agent cache for all remote state needed: root and leaf certs, intentions,
//     and service discovery results for the specified upstreams. This ensures
//     these results are always in local cache for "pull" based proxies like the
//     built-in one.
//  5. If needed, pull-based proxy config APIs like the xDS server can Watch the
//     config for a given proxy service.
//  6. Watchers get notified every time something changes the current snapshot
//     of config for the proxy. That might be changes to the registration,
//     certificate rotations, changes to the upstreams required (needing
//     different listener config), or changes to the service discovery results
//     for any upstream (e.g. new instance of upstream service came up).
package proxycfg
