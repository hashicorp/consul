package structs

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
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

// NewProxyExecMode returns the proper ProxyExecMode for the given string value.
func NewProxyExecMode(raw string) (ProxyExecMode, error) {
	switch raw {
	case "":
		return ProxyExecModeUnspecified, nil
	case "daemon":
		return ProxyExecModeDaemon, nil
	case "script":
		return ProxyExecModeScript, nil
	default:
		return 0, fmt.Errorf("invalid exec mode: %s", raw)
	}
}

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

// ConnectManagedProxy represents the agent-local state for a configured proxy
// instance. This is never stored or sent to the servers and is only used to
// store the config for the proxy that the agent needs to track. For now it's
// really generic with only the fields the agent needs to act on defined while
// the rest of the proxy config is passed as opaque bag of attributes to support
// arbitrary config params for third-party proxy integrations. "External"
// proxies by definition register themselves and manage their own config
// externally so are never represented in agent state.
type ConnectManagedProxy struct {
	// ExecMode is one of daemon or script.
	ExecMode ProxyExecMode

	// Command is the command to execute. Empty defaults to self-invoking the same
	// consul binary with proxy subcomand for ProxyExecModeDaemon and is an error
	// for ProxyExecModeScript.
	Command []string

	// Config is the arbitrary configuration data provided with the registration.
	Config map[string]interface{}

	// ProxyService is a pointer to the local proxy's service record for
	// convenience. The proxies ID and name etc. can be read from there. It may be
	// nil if the agent is starting up and hasn't registered the service yet. We
	// ignore it when calculating the hash value since the only thing that effects
	// the proxy's config is the ID of the target service which is already
	// represented below.
	ProxyService *NodeService `hash:"ignore"`

	// TargetServiceID is the ID of the target service on the localhost. It may
	// not exist yet since bootstrapping is allowed to happen in either order.
	TargetServiceID string
}

// ConnectManagedProxyConfig represents the parts of the proxy config the agent
// needs to understand. It's bad UX to make the user specify these separately
// just to make parsing simpler for us so this encapsulates the fields in
// ConnectManagedProxy.Config that we care about. They are all optional anyway
// and this is used to decode them with mapstructure.
type ConnectManagedProxyConfig struct {
	BindAddress string `mapstructure:"bind_address"`
	BindPort    int    `mapstructure:"bind_port"`
}

// ParseConfig attempts to read the fields we care about from the otherwise
// opaque config map. They are all optional but it may fail if one is specified
// but an invalid value.
func (p *ConnectManagedProxy) ParseConfig() (*ConnectManagedProxyConfig, error) {
	var cfg ConnectManagedProxyConfig
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused:      false,
		WeaklyTypedInput: true, // allow string port etc.
		Result:           &cfg,
	})
	if err != nil {
		return nil, err
	}
	err = d.Decode(p.Config)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
