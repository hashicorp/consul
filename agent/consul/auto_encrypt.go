package consul

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
)

const (
	dummyTrustDomain  = "dummy.trustdomain"
	retryJitterWindow = 30 * time.Second
)

func (c *Client) RequestAutoEncryptCerts(servers []string, port int, token string, interruptCh chan struct{}) (*structs.SignedResponse, string, error) {
	errFn := func(err error) (*structs.SignedResponse, string, error) {
		return nil, "", err
	}

	// Check if we know about a server already through gossip. Depending on
	// how the agent joined, there might already be one. Also in case this
	// gets called because the cert expired.
	server := c.routers.FindServer()
	if server != nil {
		servers = []string{server.Addr.String()}
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

	conf, err := c.config.CAConfig.GetCommonConfig()
	if err != nil {
		return errFn(err)
	}

	if conf.PrivateKeyType == "" {
		conf.PrivateKeyType = connect.DefaultPrivateKeyType
	}
	if conf.PrivateKeyBits == 0 {
		conf.PrivateKeyBits = connect.DefaultPrivateKeyBits
	}

	// Create a new private key
	pk, pkPEM, err := connect.GeneratePrivateKeyWithConfig(conf.PrivateKeyType, conf.PrivateKeyBits)
	if err != nil {
		return errFn(err)
	}

	dnsNames := []string{"localhost"}
	ipAddresses := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::")}

	// Create a CSR.
	//
	// The Common Name includes the dummy trust domain for now but Server will
	// override this when it is signed anyway so it's OK.
	cn := connect.AgentCN(string(c.config.NodeName), dummyTrustDomain)
	csr, err := connect.CreateCSR(id, cn, pk, dnsNames, ipAddresses)
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
	var reply structs.SignedResponse

	// Retry implementation modeled after https://github.com/hashicorp/consul/pull/5228.
	// TLDR; there is a 30s window from which a random time is picked.
	// Repeat until the call is successful.
	attempts := 0
	for {
		select {
		case <-interruptCh:
			return errFn(fmt.Errorf("aborting AutoEncrypt because interrupted"))
		default:
		}

		// Translate host to net.TCPAddr to make life easier for
		// RPCInsecure.
		for _, s := range servers {
			ips, err := resolveAddr(s, c.logger)
			if err != nil {
				c.logger.Warn("AutoEncrypt resolveAddr failed", "error", err)
				continue
			}

			for _, ip := range ips {
				addr := net.TCPAddr{IP: ip, Port: port}

				if err = c.connPool.RPC(c.config.Datacenter, c.config.NodeName, &addr, 0, "AutoEncrypt.Sign", true, &args, &reply); err == nil {
					return &reply, pkPEM, nil
				} else {
					c.logger.Warn("AutoEncrypt failed", "error", err)
				}
			}
		}
		attempts++

		delay := lib.RandomStagger(retryJitterWindow)
		interval := (time.Duration(attempts) * delay) + delay
		c.logger.Warn("retrying AutoEncrypt", "retry_interval", interval)
		select {
		case <-time.After(interval):
			continue
		case <-interruptCh:
			return errFn(fmt.Errorf("aborting AutoEncrypt because interrupted"))
		case <-c.shutdownCh:
			return errFn(fmt.Errorf("aborting AutoEncrypt because shutting down"))
		}
	}
}

func missingPortError(host string, err error) bool {
	return err != nil && err.Error() == fmt.Sprintf("address %s: missing port in address", host)
}

// resolveAddr is used to resolve the host into IPs and error.
func resolveAddr(rawHost string, logger hclog.Logger) ([]net.IP, error) {
	host, _, err := net.SplitHostPort(rawHost)
	if err != nil {
		// In case we encounter this error, we proceed with the
		// rawHost. This is fine since -start-join and -retry-join
		// take only hosts anyways and this is an expected case.
		if missingPortError(rawHost, err) {
			host = rawHost
		} else {
			return nil, err
		}
	}

	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}

	// First try TCP so we have the best chance for the largest list of
	// hosts to join. If this fails it's not fatal since this isn't a standard
	// way to query DNS, and we have a fallback below.
	if ips, err := tcpLookupIP(host, logger); err != nil {
		logger.Debug("TCP-first lookup failed for host, falling back to UDP", "host", host, "error", err)
	} else if len(ips) > 0 {
		return ips, nil
	}

	// If TCP didn't yield anything then use the normal Go resolver which
	// will try UDP, then might possibly try TCP again if the UDP response
	// indicates it was truncated.
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	return ips, nil
}

// tcpLookupIP is a helper to initiate a TCP-based DNS lookup for the given host.
// The built-in Go resolver will do a UDP lookup first, and will only use TCP if
// the response has the truncate bit set, which isn't common on DNS servers like
// Consul's. By doing the TCP lookup directly, we get the best chance for the
// largest list of hosts to join. Since joins are relatively rare events, it's ok
// to do this rather expensive operation.
func tcpLookupIP(host string, logger hclog.Logger) ([]net.IP, error) {
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
				logger.Debug("Ignoring CNAME RR in TCP-first answer for host", "host", host)
			}
		}
		return ips, nil
	}

	return nil, nil
}
