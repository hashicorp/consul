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
