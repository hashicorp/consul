package main

import (
	"flag"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddy/caddymain"
)

//go:generate go run plugin_generate.go

func main() {
	// Set some flags/options specific for CoreDNS.
	flag.Set("type", "dns")
	caddy.DefaultConfigFile = "Corefile"

	caddymain.Run()
}
