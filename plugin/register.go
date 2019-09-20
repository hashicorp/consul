package plugin

import "github.com/caddyserver/caddy"

// Register registers your plugin with CoreDNS and allows it to be called when the server is running.
func Register(name string, action caddy.SetupFunc) {
	caddy.RegisterPlugin(name, caddy.Plugin{
		ServerType: "dns",
		Action:     action,
	})
}
