package rcode

import (
	"strconv"

	"github.com/miekg/dns"
)

// ToString convert the rcode to the offical DNS string, or to "RCODE"+value if the RCODE
// value is unknown.
func ToString(rcode int) string {
	if str, ok := dns.RcodeToString[rcode]; ok {
		return str
	}
	return "RCODE" + strconv.Itoa(rcode)
}
