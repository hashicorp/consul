package etcd

import (
	"strings"

	"github.com/miekg/coredns/middleware/etcd/msg"

	"github.com/miekg/dns"
)

const debugName = "o-o.debug."

// isDebug checks if name is a debugging name, i.e. starts with o-o.debug.
// it return the empty string if it is not a debug message, otherwise it will return the
// name with o-o.debug. stripped off.
func isDebug(name string) string {
	if len(name) == len(debugName) {
		return ""
	}
	debug := strings.HasPrefix(name, debugName)
	if !debug {
		return ""
	}
	return name[len(debugName):]
}

// servicesToTxt puts debug in TXT RRs.
func servicesToTxt(debug []msg.Service) []dns.RR {
	if debug == nil {
		return nil
	}

	rr := make([]dns.RR, len(debug))
	for i, d := range debug {
		rr[i] = d.RR()
	}
	return rr
}
