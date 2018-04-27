// Package proxy contains logic for agent interaction with proxies,
// primarily "managed" proxies. Managed proxies are proxy processes for
// Connect-compatible endpoints that Consul owns and controls the lifecycle
// for.
//
// This package does not contain the built-in proxy for Connect. The source
// for that is available in the "connect/proxy" package.
package proxy

// EnvProxyToken is the name of the environment variable that is passed
// to managed proxies containing the proxy token.
const EnvProxyToken = "CONNECT_PROXY_TOKEN"

// Proxy is the interface implemented by all types of managed proxies.
//
// Calls to all the functions on this interface must be concurrency safe.
// Please read the documentation carefully on top of each function for expected
// behavior.
type Proxy interface {
	// Start starts the proxy. If an error is returned then the managed
	// proxy registration is rejected. Therefore, this should only fail if
	// the configuration of the proxy itself is irrecoverable, and should
	// retry starting for other failures.
	Start() error

	// Stop stops the proxy.
	Stop() error
}
