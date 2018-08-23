// Package proxy contains logic for agent interaction with proxies,
// primarily "managed" proxies. Managed proxies are proxy processes for
// Connect-compatible endpoints that Consul owns and controls the lifecycle
// for.
//
// This package does not contain the built-in proxy for Connect. The source
// for that is available in the "connect/proxy" package.
package proxy

import (
	"github.com/hashicorp/consul/agent/structs"
)

const (
	// EnvProxyID is the name of the environment variable that is set for
	// managed proxies containing the proxy service ID. This is required along
	// with the token to make API requests related to the proxy.
	EnvProxyID = "CONNECT_PROXY_ID"

	// EnvProxyToken is the name of the environment variable that is passed
	// to managed proxies containing the proxy token.
	EnvProxyToken = "CONNECT_PROXY_TOKEN"
)

// Proxy is the interface implemented by all types of managed proxies.
//
// Calls to all the functions on this interface must be concurrency safe.
// Please read the documentation carefully on top of each function for expected
// behavior.
//
// Whenever a new proxy type is implemented, please also update proxyExecMode
// and newProxyFromMode and newProxy to support the new proxy.
type Proxy interface {
	// Start starts the proxy. If an error is returned then the managed
	// proxy registration is rejected. Therefore, this should only fail if
	// the configuration of the proxy itself is irrecoverable, and should
	// retry starting for other failures.
	//
	// Starting an already-started proxy should not return an error.
	Start() error

	// Stop stops the proxy and disallows it from ever being started again.
	// This should also clean up any resources used by this Proxy.
	//
	// If the proxy is not started yet, this should not return an error, but
	// it should disallow Start from working again. If the proxy is already
	// stopped, this should not return an error.
	Stop() error

	// Close should clean up any resources associated with this proxy but
	// keep it running in the background. Only one of Close or Stop can be
	// called.
	Close() error

	// Equal returns true if the argument is equal to the proxy being called.
	// This is called by the manager to determine if a change in configuration
	// results in a proxy that needs to be restarted or not. If Equal returns
	// false, then the manager will stop the old proxy and start the new one.
	// If Equal returns true, the old proxy will remain running and the new
	// one will be ignored.
	Equal(Proxy) bool

	// MarshalSnapshot returns the state that will be stored in a snapshot
	// so that Consul can recover the proxy process after a restart. The
	// result should only contain primitive values and containers (lists/maps).
	//
	// MarshalSnapshot does NOT need to store the following fields, since they
	// are part of the manager snapshot and will be automatically restored
	// for any proxies: proxy ID.
	//
	// UnmarshalSnapshot is called to restore the receiving Proxy from its
	// marshalled state. If UnmarshalSnapshot returns an error, the snapshot
	// is ignored and the marshalled snapshot will be lost. The manager will
	// log.
	//
	// This should save/restore enough state to be able to regain management
	// of a proxy process as well as to perform the Equal method above. The
	// Equal method will be called when a local state sync happens to determine
	// if the recovered process should be restarted or not.
	MarshalSnapshot() map[string]interface{}
	UnmarshalSnapshot(map[string]interface{}) error
}

// proxyExecMode returns the ProxyExecMode for a Proxy instance.
func proxyExecMode(p Proxy) structs.ProxyExecMode {
	switch p.(type) {
	case *Daemon:
		return structs.ProxyExecModeDaemon

	case *Noop:
		return structs.ProxyExecModeTest

	default:
		return structs.ProxyExecModeUnspecified
	}
}
