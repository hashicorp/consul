package plugin

import (
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/go-plugin"
)

// Name is the name of the plugin that users of the package should use
// with *plugin.Client.Dispense to get the proper plugin instance.
const Name = "consul-connect-ca"

// handshakeConfig is the HandshakeConfig used to configure clients and servers.
var handshakeConfig = plugin.HandshakeConfig{
	// The ProtocolVersion is the version that must match between Consul
	// and CA plugins. This should be bumped whenever a change happens in
	// one or the other that makes it so that they can't safely communicate.
	ProtocolVersion: 1,

	// The magic cookie values should NEVER be changed.
	MagicCookieKey:   "CONSUL_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "f31f63b28fa82a3cdb30a6284cb1e50e3a13b7e60ba105a2c91219da319d216c",
}

// Serve serves a CA plugin. This function never returns and should be the
// final function called in the main function of the plugin.
func Serve(p ca.Provider) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins: map[string]plugin.Plugin{
			Name: &ProviderPlugin{Impl: p},
		},
	})
}
