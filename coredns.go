package main

import (
	"flag"

	"github.com/mholt/caddy"
	"github.com/mholt/caddy/caddy/caddymain"
)

//go:generate go run plugin_generate.go

func main() {
	setFlag()
	setName()

	caddymain.Run()
}

// setFlag sets flags to predefined values for CoreDNS.
func setFlag() {
	flag.Set("type", "dns")
}

// setName sets application name and versioning information for CoreDNS.
func setName() {
	caddy.DefaultConfigFile = "Corefile"
	caddy.AppName = "CoreDNS"
	caddy.AppVersion = version
}

const version = "001"
