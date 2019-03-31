package main

//go:generate go run directives_generate.go
//go:generate go run owners_generate.go

import (
	"github.com/coredns/coredns/coremain"

	// Plug in CoreDNS
	_ "github.com/coredns/coredns/core/plugin"
)

func main() {
	coremain.Run()
}
