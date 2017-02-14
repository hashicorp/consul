package main

//go:generate go run directives_generate.go

import "github.com/miekg/coredns/coremain"

func main() {
	coremain.Run()
}
