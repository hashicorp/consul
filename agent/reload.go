package agent

import "github.com/hashicorp/consul/agent/config"

// ConfigReloader is a function type which may be implemented to support reloading
// of configuration.
type ConfigReloader func(rtConfig *config.RuntimeConfig) error

// reloadConfigDNSServer returns a closure that translates RuntimeConfig into
// dnsConfig and then calls DNSServer.ReloadConfig.
// This function exists so that all references to RuntimeConfig stay in the
// agent package. In the future the DNSServer would be moved out into a separate
// package which should not reference RuntimeConfig. This is done in advance as
// a demonstration of this pattern, and so that config reload looks consistent
// across all components.
func reloadConfigDNSServer(s *DNSServer) func(runtimeConfig *config.RuntimeConfig) error {
	return func(rtConfig *config.RuntimeConfig) error {
		cfg, err := newDNSConfig(rtConfig)
		if err != nil {
			return err
		}
		return s.ReloadConfig(cfg)
	}
}
