package etcd

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/coredns/middleware/proxy"

	"github.com/miekg/dns"
)

func (e Etcd) UpdateStubZones() {
	go func() {
		for {
			e.updateStubZones()
			time.Sleep(15 * time.Second)
		}
	}()
}

// Look in .../dns/stub/<zone>/xx for msg.Services. Loop through them
// extract <zone> and add them as forwarders (ip:port-combos) for
// the stub zones. Only numeric (i.e. IP address) hosts are used.
func (e Etcd) updateStubZones() {
	stubmap := make(map[string]proxy.Proxy)
	for _, zone := range e.Zones {
		services, err := e.Records(stubDomain+"."+zone, false)
		if err != nil {
			continue
		}

		// track the nameservers on a per domain basis, but allow a list on the domain.
		nameservers := map[string][]string{}

		for _, serv := range services {
			if serv.Port == 0 {
				serv.Port = 53
			}
			ip := net.ParseIP(serv.Host)
			if ip == nil {
				continue
			}

			domain := e.Domain(serv.Key)
			labels := dns.SplitDomainName(domain)
			// nameserver need to be tracked by domain and *then* added

			// If the remaining name equals any of the zones we have, we ignore it.
			for _, z := range e.Zones {
				// Chop of left most label, because that is used as the nameserver place holder
				// and drop the right most labels that belong to zone.
				domain = dns.Fqdn(strings.Join(labels[1:len(labels)-dns.CountLabel(z)], "."))
				if domain == z {
					continue
				}
				nameservers[domain] = append(nameservers[domain], net.JoinHostPort(serv.Host, strconv.Itoa(serv.Port)))
			}
		}
		for domain, nss := range nameservers {
			stubmap[domain] = proxy.New(nss)
		}
	}

	// atomic swap (at least that's what we hope it is)
	if len(stubmap) > 0 {
		e.Stubmap = &stubmap
	}
	return
}
