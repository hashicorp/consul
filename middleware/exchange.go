package middleware

import "github.com/miekg/dns"

func Exchange(c *dns.Client, m *dns.Msg, server string) (*dns.Msg, error) {
	r, _, err := c.Exchange(m, server)
	return r, err
}
