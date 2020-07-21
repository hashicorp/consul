package certmon

import (
	"context"
	"net"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-hclog"
)

// FallbackFunc is used when the normal cache watch based Certificate
// updating fails to update the Certificate in time and a different
// method of updating the certificate is required.
type FallbackFunc func(context.Context) (*structs.SignedResponse, error)

type Config struct {
	// Logger is the logger to be used while running. If not set
	// then no logging will be performed.
	Logger hclog.Logger

	// TLSConfigurator is where the certificates and roots are set when
	// they are updated. This field is required.
	TLSConfigurator *tlsutil.Configurator

	// Cache is an object implementing our Cache interface. The Cache
	// used at runtime must be able to handle Roots and Leaf Cert watches
	Cache Cache

	// Tokens is the shared token store. It is used to retrieve the current
	// agent token as well as getting notifications when that token is updated.
	// This field is required.
	Tokens *token.Store

	// Fallback is a function to run when the normal cache updating of the
	// agent's certificates has failed to work for one reason or another.
	// This field is required.
	Fallback FallbackFunc

	// FallbackLeeway is the amount of time after certificate expiration before
	// invoking the fallback routine. If not set this will default to 10s.
	FallbackLeeway time.Duration

	// FallbackRetry is the duration between Fallback invocations when the configured
	// fallback routine returns an error. If not set this will default to 1m.
	FallbackRetry time.Duration

	// DNSSANs is a list of DNS SANs that certificate requests should include. This
	// field is optional and no extra DNS SANs will be requested if unset. 'localhost'
	// is unconditionally requested by the cache implementation.
	DNSSANs []string

	// IPSANs is a list of IP SANs to include in the certificate signing request. This
	// field is optional and no extra IP SANs will be requested if unset. Both '127.0.0.1'
	// and '::1' IP SANs are unconditionally requested by the cache implementation.
	IPSANs []net.IP

	// Datacenter is the datacenter to request certificates within. This filed is required
	Datacenter string

	// NodeName is the agent's node name to use when requesting certificates. This field
	// is required.
	NodeName string
}

// WithCache will cause the created CertMonitor type to use the provided Cache
func (cfg *Config) WithCache(cache Cache) *Config {
	cfg.Cache = cache
	return cfg
}

// WithLogger will cause the created CertMonitor type to use the provided logger
func (cfg *Config) WithLogger(logger hclog.Logger) *Config {
	cfg.Logger = logger
	return cfg
}

// WithTLSConfigurator will cause the created CertMonitor type to use the provided configurator
func (cfg *Config) WithTLSConfigurator(tlsConfigurator *tlsutil.Configurator) *Config {
	cfg.TLSConfigurator = tlsConfigurator
	return cfg
}

// WithTokens will cause the created CertMonitor type to use the provided token store
func (cfg *Config) WithTokens(tokens *token.Store) *Config {
	cfg.Tokens = tokens
	return cfg
}

// WithFallback configures a fallback function to use if the normal update mechanisms
// fail to renew the certificate in time.
func (cfg *Config) WithFallback(fallback FallbackFunc) *Config {
	cfg.Fallback = fallback
	return cfg
}

// WithDNSSANs configures the CertMonitor to request these DNS SANs when requesting a new
// certificate
func (cfg *Config) WithDNSSANs(sans []string) *Config {
	cfg.DNSSANs = sans
	return cfg
}

// WithIPSANs configures the CertMonitor to request these IP SANs when requesting a new
// certificate
func (cfg *Config) WithIPSANs(sans []net.IP) *Config {
	cfg.IPSANs = sans
	return cfg
}

// WithDatacenter configures the CertMonitor to request Certificates in this DC
func (cfg *Config) WithDatacenter(dc string) *Config {
	cfg.Datacenter = dc
	return cfg
}

// WithNodeName configures the CertMonitor to request Certificates with this agent name
func (cfg *Config) WithNodeName(name string) *Config {
	cfg.NodeName = name
	return cfg
}

// WithFallbackLeeway configures how long after a certificate expires before attempting to
// generarte a new certificate using the fallback mechanism. The default is 10s.
func (cfg *Config) WithFallbackLeeway(leeway time.Duration) *Config {
	cfg.FallbackLeeway = leeway
	return cfg
}

// WithFallbackRetry controls how quickly we will make subsequent invocations of
// the fallback func in the case of it erroring out.
func (cfg *Config) WithFallbackRetry(after time.Duration) *Config {
	cfg.FallbackRetry = after
	return cfg
}
