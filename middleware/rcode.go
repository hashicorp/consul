package middleware

import (
	"strconv"

	"github.com/miekg/dns"
)

func RcodeToString(rcode int) string {
	if str, ok := dns.RcodeToString[rcode]; ok {
		return str
	}
	return "RCODE" + strconv.Itoa(rcode)
}
