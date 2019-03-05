package plugin

import (
	"github.com/hashicorp/go-plugin"
)

// ClientConfig returns a base *plugin.ClientConfig that is configured to
// be able to dispense CA provider plugins. The returned value should be
// modified with additional options prior to execution (such as Cmd, Managed,
// etc.)
func ClientConfig() *plugin.ClientConfig {
	return &plugin.ClientConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			Name: &ProviderPlugin{},
		},
	}
}
