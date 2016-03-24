package etcd

import (
	"net"
	"strconv"
	"strings"

	"github.com/miekg/dns"
)

// hasStubEdns0 checks if the message is carrying our special
// edns0 zero option.
func hasStubEdns0(m *dns.Msg) bool {
	option := m.IsEdns0()
	if option == nil {
		return false
	}
	for _, o := range option.Option {
		if o.Option() == ednsStubCode && len(o.(*dns.EDNS0_LOCAL).Data) == 1 &&
			o.(*dns.EDNS0_LOCAL).Data[0] == 1 {
			return true
		}
	}
	return false
}

// addStubEdns0 adds our special option to the message's OPT record.
func addStubEdns0(m *dns.Msg) *dns.Msg {
	option := m.IsEdns0()
	// Add a custom EDNS0 option to the packet, so we can detect loops
	// when 2 stubs are forwarding to each other.
	if option != nil {
		option.Option = append(option.Option, &dns.EDNS0_LOCAL{ednsStubCode, []byte{1}})
	} else {
		m.Extra = append(m.Extra, ednsStub)
	}
	return m
}

// Look in .../dns/stub/<domain>/xx for msg.Services. Loop through them
// extract <domain> and add them as forwarders (ip:port-combos) for
// the stub zones. Only numeric (i.e. IP address) hosts are used.
// TODO(miek): makes this Startup Function.
func (e Etcd) UpdateStubZones(zone string) error {
	stubmap := make(map[string][]string)

	services, err := e.Records("stub.dns."+zone, false)
	if err != nil {
		return err
	}
	for _, serv := range services {
		if serv.Port == 0 {
			serv.Port = 53
		}
		ip := net.ParseIP(serv.Host)
		if ip == nil {
			//logf("stub zone non-address %s seen for: %s", serv.Key, serv.Host)
			continue
		}

		domain := e.Domain(serv.Key)
		labels := dns.SplitDomainName(domain)

		// If the remaining name equals any of the zones we have, we ignore it.
		for _, z := range e.Zones {
			// Chop of left most label, because that is used as the nameserver place holder
			// and drop the right most labels that belong to zone.
			domain = dns.Fqdn(strings.Join(labels[1:len(labels)-dns.CountLabel(z)], "."))
			if domain == z {
				continue
			}
			stubmap[domain] = append(stubmap[domain], net.JoinHostPort(serv.Host, strconv.Itoa(serv.Port)))
		}
	}

	// TODO(miek): add to etcd structure and startup with a StartFunction
	//	e.stub = &stubmap
	// stubmap contains proxy is best way forward... I think.
	// TODO(miek): setup a proxy that forward to these
	// StubProxy type?
	return nil
}

func ServeDNSStubForward(w dns.ResponseWriter, req *dns.Msg) *dns.Msg {
	if !hasStubEdns0(req) {
		return nil
	}
	req = addStubEdns0(req)
	// proxy woxy
	return nil
}

// ednsStub is the EDNS0 record we add to stub queries. Queries which have this record are
// not forwarded again.
var ednsStub = func() *dns.OPT {
	o := new(dns.OPT)
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT

	e := new(dns.EDNS0_LOCAL)
	e.Code = ednsStubCode
	e.Data = []byte{1}
	o.Option = append(o.Option, e)
	return o
}()

const ednsStubCode = dns.EDNS0LOCALSTART + 10
