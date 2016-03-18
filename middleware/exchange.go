package middleware

import "github.com/miekg/dns"

// Exchang sends message m to the server.
// TODO(miek): optionally it can do retries of other silly stuff.
func Exchange(c *dns.Client, m *dns.Msg, server string) (*dns.Msg, error) {
	r, _, err := c.Exchange(m, server)
	return r, err
}
