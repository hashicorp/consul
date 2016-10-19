package test

import (
	"io/ioutil"
	"log"
	"sort"
	"testing"

	"github.com/miekg/coredns/middleware/proxy"
	mtest "github.com/miekg/coredns/middleware/test"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
)

// Using miek.nl here because this is the easiest zone to get access to and it's masters
// run both NSD and BIND9, making checks like "what should we actually return" super easy.
var dsTestCases = []mtest.Case{
	{
		Qname: "_udp.miek.nl.", Qtype: dns.TypeDS,
		Rcode: dns.RcodeNameError,
		Ns: []dns.RR{
			mtest.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
	{
		Qname: "miek.nl.", Qtype: dns.TypeDS,
		Ns: []dns.RR{
			mtest.SOA("miek.nl.	1800	IN	SOA	linode.atoom.net. miek.miek.nl. 1282630057 14400 3600 604800 14400"),
		},
	},
}

func TestLookupDS(t *testing.T) {
	name, rm, err := TempFile(".", miekNL)
	if err != nil {
		t.Fatalf("failed to created zone: %s", err)
	}
	defer rm()

	corefile := `miek.nl:0 {
       file ` + name + `
}
`

	i, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(i, 0)
	if udp == "" {
		t.Fatalf("Could not get UDP listening port")
	}
	defer i.Stop()

	log.SetOutput(ioutil.Discard)

	p := proxy.New([]string{udp})
	state := request.Request{W: &mtest.ResponseWriter{}, Req: new(dns.Msg)}

	for _, tc := range dsTestCases {
		resp, err := p.Lookup(state, tc.Qname, tc.Qtype)
		if err != nil || resp == nil {
			t.Fatalf("Expected to receive reply, but didn't for %s %d", tc.Qname, tc.Qtype)
		}

		sort.Sort(mtest.RRSet(resp.Answer))
		sort.Sort(mtest.RRSet(resp.Ns))
		sort.Sort(mtest.RRSet(resp.Extra))

		if !mtest.Header(t, tc, resp) {
			t.Logf("%v\n", resp)
			continue
		}
		if !mtest.Section(t, tc, mtest.Answer, resp.Answer) {
			t.Logf("%v\n", resp)
		}
		if !mtest.Section(t, tc, mtest.Ns, resp.Ns) {
			t.Logf("%v\n", resp)
		}
		if !mtest.Section(t, tc, mtest.Extra, resp.Extra) {
			t.Logf("%v\n", resp)
		}
	}
}
