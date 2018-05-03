package proxy

import (
	"github.com/hashicorp/consul/agent/structs"
)

// snapshot is the structure of the snapshot file. This is unexported because
// we don't want this being a public API.
//
// The snapshot doesn't contain any configuration for the manager. We only
// want to restore the proxies that we're managing, and we use the config
// set at runtime to sync and reconcile what proxies we should start,
// restart, stop, or have already running.
type snapshot struct {
	// Version is the version of the snapshot format and can be used
	// to safely update the format in the future if necessary.
	Version int

	// Proxies are the set of proxies that the manager has.
	Proxies []snapshotProxy
}

// snapshotProxy represents a single proxy.
type snapshotProxy struct {
	// Mode corresponds to the type of proxy running.
	Mode structs.ProxyExecMode

	// Config is an opaque mapping of primitive values that the proxy
	// implementation uses to restore state.
	Config map[string]interface{}
}
