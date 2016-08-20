package bind

import "github.com/mholt/caddy"

func init() {
	caddy.RegisterPlugin("bind", caddy.Plugin{
		ServerType: "dns",
		Action:     setupBind,
	})
}
