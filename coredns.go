package main

import (
	"flag"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddy/caddymain"
)

//go:generate go run plugin_generate.go

func main() {
	// Default values for flags for CoreDNS.
	flag.Set("type", "dns")

	// Values specific for CoreDNS.
	caddy.DefaultConfigFile = "Corefile"
	caddy.AppName = "coredns"
	caddy.AppVersion = version

	caddymain.Run()
}

const version = "001"
