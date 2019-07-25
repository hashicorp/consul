package structs

import (
	"fmt"
)

// ConnectAuthorizeRequest is the structure of a request to authorize
// a connection.
type ConnectAuthorizeRequest struct {
	// Target is the name of the service that is being requested.
	Target string

	// ClientCertURI is a unique identifier for the requesting client. This
	// is currently the URI SAN from the TLS client certificate.
	//
	// ClientCertSerial is a colon-hex-encoded of the serial number for
	// the requesting client cert. This is used to check against revocation
	// lists.
	ClientCertURI    string
	ClientCertSerial string
}

// ProxyExecMode encodes the mode for running a managed connect proxy.
type ProxyExecMode int

const (
	// ProxyExecModeUnspecified uses the global default proxy mode.
	ProxyExecModeUnspecified ProxyExecMode = iota

	// ProxyExecModeDaemon executes a proxy process as a supervised daemon.
	ProxyExecModeDaemon

	// ProxyExecModeScript executes a proxy config script on each change to it's
	// config.
	ProxyExecModeScript

	// ProxyExecModeTest tracks the start/stop of the proxy in-memory
	// and is only used for tests. This shouldn't be set outside of tests,
	// but even if it is it has no external effect.
	ProxyExecModeTest
)

// String implements Stringer
func (m ProxyExecMode) String() string {
	switch m {
	case ProxyExecModeUnspecified:
		return "global_default"
	case ProxyExecModeDaemon:
		return "daemon"
	case ProxyExecModeScript:
		return "script"
	case ProxyExecModeTest:
		return "test"
	default:
		return "unknown"
	}
}
