package consul

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/miekg/dns"
)

const (
	dummyTrustDomain  = "dummy.trustdomain"
	retryJitterWindow = 30 * time.Second
)

func (c *Client) AutoEncrypt(servers []string, port int, token string) (*structs.SignResponse, string, error) {
	errFn := func(err error) (*structs.SignResponse, string, error) {
		return nil, "", err
	}

	if len(servers) == 0 {
		return errFn(fmt.Errorf("No servers to request AutoEncrypt.Sign"))
	}

	// We don't provide the correct host here, because we don't know any
	// better at this point. Apart from the domain, we would need the
	// ClusterID, which we don't have. This is why we go with
	// dummyTrustDomain the first time. Subsequent CSRs will have the
	// correct TrustDomain.
	id := &connect.SpiffeIDAgent{
		Host:       dummyTrustDomain,
		Datacenter: c.config.Datacenter,
		Agent:      string(c.config.NodeName),
	}

	// Create a new private key
	pk, pkPEM, err := connect.GeneratePrivateKey()
	if err != nil {
		return errFn(err)
	}

	// Create a CSR.
	csr, err := connect.CreateCSR(id, pk)
	if err != nil {
		return errFn(err)
	}

	// Prepare request and response so that it can be passed to
	// RPCInsecure.
	args := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: token},
		Datacenter:   c.config.Datacenter,
		CSR:          csr,
	}
	var reply structs.SignResponse

	// Retry implementation modeled after https://github.com/hashicorp/consul/pull/5228.
	// TLDR; there is a 30s window from which a random time is picked.
	// Repeat until the call is successful.
	attempts := 0
	for {
		// Translate host to net.TCPAddr to make life easier for
		// RPCInsecure.
		addrs := []*net.TCPAddr{}
		for _, s := range servers {
			ips, err := resolveAddr(s, c.logger)
			if err != nil {
				c.logger.Printf("[WARN] agent: AutoEncrypt resolveAddr failed: %v", err)
				continue
			}
			for _, ip := range ips {
				addrs = append(addrs, &net.TCPAddr{IP: ip, Port: port})
			}
		}

		if err = c.RPCInsecure("AutoEncrypt.Sign", &args, &reply, addrs); err == nil {
			return &reply, pkPEM, nil
		}

		delay := lib.RandomStagger(retryJitterWindow)
		interval := (time.Duration(attempts) * delay) + delay
		c.logger.Printf("[WARN] agent: AutoEncrypt failed: %v, retrying in %v", err, interval)
		select {
		case <-time.After(interval):
			continue
		case <-c.shutdownCh:
			return errFn(fmt.Errorf("aborting AutoEncrypt because shutting down"))
		}
	}
}

// resolveAddr is used to resolve the address into an address,
// port, and error. If no port is given, use the default
func resolveAddr(rawHost string, logger *log.Logger) ([]net.IP, error) {
	host, _, err := net.SplitHostPort(rawHost)
	if err != nil && err.Error() != "missing port in address" {
		return nil, err
	}

	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	// First try TCP so we have the best chance for the largest list of
	// hosts to join. If this fails it's not fatal since this isn't a standard
	// way to query DNS, and we have a fallback below.
	if ips, err := tcpLookupIP(host, logger); err != nil {
		logger.Printf("[DEBUG] memberlist: TCP-first lookup failed for '%s', falling back to UDP: %s", host, err)
	} else if len(ips) > 0 {
		return ips, nil
	}

	// If TCP didn't yield anything then use the normal Go resolver which
	// will try UDP, then might possibly try TCP again if the UDP response
	// indicates it was truncated.
	return net.LookupIP(host)
}

// tcpLookupIP is a helper to initiate a TCP-based DNS lookup for the given host.
// The built-in Go resolver will do a UDP lookup first, and will only use TCP if
// the response has the truncate bit set, which isn't common on DNS servers like
// Consul's. By doing the TCP lookup directly, we get the best chance for the
// largest list of hosts to join. Since joins are relatively rare events, it's ok
// to do this rather expensive operation.
func tcpLookupIP(host string, logger *log.Logger) ([]net.IP, error) {
	// Don't attempt any TCP lookups against non-fully qualified domain
	// names, since those will likely come from the resolv.conf file.
	if !strings.Contains(host, ".") {
		return nil, nil
	}

	// Make sure the domain name is terminated with a dot (we know there's
	// at least one character at this point).
	dn := host
	if dn[len(dn)-1] != '.' {
		dn = dn + "."
	}

	// See if we can find a server to try.
	cc, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}
	if len(cc.Servers) > 0 {
		// Do the lookup.
		c := new(dns.Client)
		c.Net = "tcp"
		msg := new(dns.Msg)
		msg.SetQuestion(dn, dns.TypeANY)
		in, _, err := c.Exchange(msg, cc.Servers[0])
		if err != nil {
			return nil, err
		}

		// Handle any IPs we get back that we can attempt to join.
		var ips []net.IP
		for _, r := range in.Answer {
			switch rr := r.(type) {
			case (*dns.A):
				ips = append(ips, rr.A)
			case (*dns.AAAA):
				ips = append(ips, rr.AAAA)
			case (*dns.CNAME):
				logger.Printf("[DEBUG] memberlist: Ignoring CNAME RR in TCP-first answer for '%s'", host)
			}
		}
		return ips, nil
	}

	return nil, nil
}
