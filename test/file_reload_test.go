package test

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/proxy"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestZoneReload(t *testing.T) {
	file.TickTime = 1 * time.Second

	name, rm, err := TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("Failed to create zone: %s", err)
	}
	defer rm()

	// Corefile with two stanzas
	corefile := `example.org:0 {
       file ` + name + `
}

example.net:0 {
	file ` + name + `
}
`
	i, udp, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer i.Stop()

	p := proxy.NewLookup([]string{udp})
	state := request.Request{W: &test.ResponseWriter{}, Req: new(dns.Msg)}

	resp, err := p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatalf("Expected to receive reply, but didn't: %s", err)
	}
	if len(resp.Answer) != 2 {
		t.Fatalf("Expected two RR in answer section got %d", len(resp.Answer))
	}

	// Remove RR from the Apex
	ioutil.WriteFile(name, []byte(exampleOrgUpdated), 0644)

	time.Sleep(2 * time.Second) // reload time

	resp, err = p.Lookup(state, "example.org.", dns.TypeA)
	if err != nil {
		t.Fatal("Expected to receive reply, but didn't")
	}

	if len(resp.Answer) != 1 {
		t.Fatalf("Expected two RR in answer section got %d", len(resp.Answer))
	}
}

const exampleOrgUpdated = `; example.org test file
example.org.		IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2016082541 7200 3600 1209600 3600
example.org.		IN	NS	b.iana-servers.net.
example.org.		IN	NS	a.iana-servers.net.
example.org.		IN	A	127.0.0.2
`
