// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package proxycfg contains components for sourcing the data required to
// configure Connect proxies. The Manager provides an API with which proxy
// services can be registered, and coordinates the fetching (and refreshing)
// of intentions, upstreams, discovery chain, certificates etc. Consumers
// such as the xDS server can then subscribe to receive snapshots of this
// data whenever it changes.
//
// Consul client agents support the configuration of proxies locally
// registered to them, whereas Consul servers support both this and proxies
// in the catalog.
//
// The following diagram depicts the component relationships on a server, as
// this is the more complex mode of operation:
//
//		               +-------+       1.       +------------+
//		               | Local | ◀------------▶ | Local      |
//		               | State |                | State Sync |
//		               +-------+                +-----+------+
//		                 ▲                            |
//	    +-------+            |     +---------------+      | 2.
//	    | envoy |         4. | 4a. | Local         |      |
//	    +-------+            | +-▶ | Config Source +-+    |
//		 | stream        | |   +---------------+ |    |
//		 ▼               | |                     ▼    ▼
//		+--------+ 3.  +-+-+-----------+ 6.    +----------+ 2a.  +----------+
//		| xDS    +---▶ | Catalog       +-----▶ | proxycfg +----▶ | proxycfg |
//		| Server | ◀---+ Config Source +-----▶ | Manager  +--+   | State    |
//		+--------+  8. +----+----------+ 7.    +----------+  |   +----------+
//		                 5. |                                |
//		                    ▼                            7a. |   +----------+
//		                  +-------+                          +-▶ | proxycfg |
//		                  | State |                              | State    |
//		                  | Store |                              +----------+
//		                  +-------+
//
//	1. local.Sync watches the agent's local state for changes.
//	2. If any sidecar proxy or gateway services are registered to the local agent
//	   they are sync'd to the proxycfg.Manager.
//	   2a. proxycfg.Manager creates a state object for the service and begins
//	       pre-fetching data (go to 8).
//	3. Client (i.e., envoy) begins a stream and the xDS server calls Watch on its
//	   ConfigSource - on a client agent this would be a local config source, on a
//	   server it would be a catalog config source.
//	4. On server, the catalog config source will check if service is registered locally.
//	   4a. If the service *is* registered locally it hands off the the local config
//	      source, which calls Watch on the proxycfg manager (and serves the pre-
//	      fetched data).
//	5. Otherwise, it fetches the service from the state store.
//	6. It calls Watch on the proxycfg manager.
//	7. It registers the service with the proxycfg manager.
//		  7a. See: 2a.
//	8. xDS server receives snapshots of configuration data whenever it changes.
package proxycfg
