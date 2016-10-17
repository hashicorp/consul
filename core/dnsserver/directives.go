package dnsserver

import (
	"fmt"
	"os"
	"strings"
)

// RegisterDevDirective splices name into the list of directives
// immediately before another directive. This function is ONLY
// for plugin development purposes! NEVER use it for a plugin
// that you are not currently building. If before is empty,
// the directive will be appended to the end of the list.
//
// It is imperative that directives execute in the proper
// order, and hard-coding the list of directives guarantees
// a correct, absolute order every time. This function is
// convenient when developing a plugin, but it does not
// guarantee absolute ordering. Multiple plugins registering
// directives with this function will lead to non-
// deterministic builds and buggy software.
//
// Directive names must be lower-cased and unique. Any errors
// here are fatal, and even successful calls print a message
// to stdout as a reminder to use it only in development.
func RegisterDevDirective(name, before string) {
	if name == "" {
		fmt.Println("[FATAL] Cannot register empty directive name")
		os.Exit(1)
	}
	if strings.ToLower(name) != name {
		fmt.Printf("[FATAL] %s: directive name must be lowercase\n", name)
		os.Exit(1)
	}
	for _, dir := range directives {
		if dir == name {
			fmt.Printf("[FATAL] %s: directive name already exists\n", name)
			os.Exit(1)
		}
	}
	if before == "" {
		directives = append(directives, name)
	} else {
		var found bool
		for i, dir := range directives {
			if dir == before {
				directives = append(directives[:i], append([]string{name}, directives[i:]...)...)
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("[FATAL] %s: directive not found\n", before)
			os.Exit(1)
		}
	}
	msg := fmt.Sprintf("Registered directive '%s' ", name)
	if before == "" {
		msg += "at end of list"
	} else {
		msg += fmt.Sprintf("before '%s'", before)
	}
	fmt.Printf("[INFO] %s\n", msg)
}

// Add here, and in core/coredns.go to use them.

// Directives are registered in the order they should be
// executed.
//
// Ordering is VERY important. Every middleware will
// feel the effects of all other middleware below
// (after) them during a request, but they must not
// care what middleware above them are doing.
var directives = []string{
	"root",
	"bind",
	"health",
	"pprof",

	"prometheus",
	"errors",
	"log",
	"chaos",
	"cache",

	"rewrite",
	"loadbalance",

	"dnssec",
	"file",
	"auto",
	"secondary",
	"etcd",
	"kubernetes",
	"proxy",
	"whoami",
}
