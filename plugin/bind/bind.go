// Package bind allows binding to a specific interface instead of bind to all of them.
package bind

import "github.com/caddyserver/caddy"

func init() {
	caddy.RegisterPlugin("bind", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}
