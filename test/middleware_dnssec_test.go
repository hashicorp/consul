package test

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/miekg/coredns/middleware/test"

	"github.com/miekg/dns"
)

func TestLookupBalanceRewriteCacheDnssec(t *testing.T) {
	name, rm, err := test.TempFile(".", exampleOrg)
	if err != nil {
		t.Fatalf("failed to created zone: %s", err)
	}
	defer rm()
	rm1 := createKeyFile(t)
	defer rm1()

	corefile := `example.org:0 {
    file ` + name + `
    rewrite ANY HINFO
    dnssec {
        key file ` + base + `
    }
    loadbalance
}
`
	ex, err := CoreDNSServer(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}

	udp, _ := CoreDNSServerPorts(ex, 0)
	defer ex.Stop()

	log.SetOutput(ioutil.Discard)
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)
	m.SetEdns0(4096, true)
	res, _, err := c.Exchange(m, udp)
	if err != nil {
		t.Fatalf("Could not send query: %s", err)
	}
	sig := 0
	for _, a := range res.Answer {
		if a.Header().Rrtype == dns.TypeRRSIG {
			sig++
		}
	}
	if sig == 0 {
		t.Errorf("expected RRSIGs, got none")
		t.Logf("%v\n", res)
	}
}

func createKeyFile(t *testing.T) func() {
	ioutil.WriteFile(base+".key",
		[]byte(`example.org. IN DNSKEY 256 3 13 tDyI0uEIDO4SjhTJh1AVTFBLpKhY3He5BdAlKztewiZ7GecWj94DOodg ovpN73+oJs+UfZ+p9zOSN5usGAlHrw==`),
		0644)
	ioutil.WriteFile(base+".private",
		[]byte(`Private-key-format: v1.3
Algorithm: 13 (ECDSAP256SHA256)
PrivateKey: HPmldSNfrkj/aDdUMFwuk/lgzaC5KIsVEG3uoYvF4pQ=
Created: 20160426083115
Publish: 20160426083115
Activate: 20160426083115`),
		0644)
	return func() {
		os.Remove(base + ".key")
		os.Remove(base + ".private")
	}
}

const base = "Kexample.org.+013+44563"
