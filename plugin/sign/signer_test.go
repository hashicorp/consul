package sign

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/caddyserver/caddy"
	"github.com/miekg/dns"
)

func TestSign(t *testing.T) {
	input := `sign testdata/db.miek.nl miek.nl {
		key file testdata/Kmiek.nl.+013+59725
		directory testdata
	}`
	c := caddy.NewTestController("dns", input)
	sign, err := parse(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(sign.signers) != 1 {
		t.Fatalf("Expected 1 signer, got %d", len(sign.signers))
	}
	z, err := sign.signers[0].Sign(time.Now().UTC())
	if err != nil {
		t.Error(err)
	}

	apex, _ := z.Search("miek.nl.")
	if x := apex.Type(dns.TypeDS); len(x) != 0 {
		t.Errorf("Expected %d DS records, got %d", 0, len(x))
	}
	if x := apex.Type(dns.TypeCDS); len(x) != 2 {
		t.Errorf("Expected %d CDS records, got %d", 2, len(x))
	}
	if x := apex.Type(dns.TypeCDNSKEY); len(x) != 1 {
		t.Errorf("Expected %d CDNSKEY record, got %d", 1, len(x))
	}
	if x := apex.Type(dns.TypeDNSKEY); len(x) != 1 {
		t.Errorf("Expected %d DNSKEY record, got %d", 1, len(x))
	}
}

func TestSignApexZone(t *testing.T) {
	apex := `$TTL    30M
$ORIGIN example.org.
@       IN      SOA     linode miek.miek.nl. ( 1282630060 4H 1H 7D 4H )
        IN      NS      linode
`
	if err := ioutil.WriteFile("db.apex-test.example.org", []byte(apex), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("db.apex-test.example.org")
	input := `sign db.apex-test.example.org example.org {
		key file testdata/Kmiek.nl.+013+59725
		directory testdata
	}`
	c := caddy.NewTestController("dns", input)
	sign, err := parse(c)
	if err != nil {
		t.Fatal(err)
	}
	z, err := sign.signers[0].Sign(time.Now().UTC())
	if err != nil {
		t.Error(err)
	}

	el, _ := z.Search("example.org.")
	nsec := el.Type(dns.TypeNSEC)
	if len(nsec) != 1 {
		t.Errorf("Expected 1 NSEC for %s, got %d", "example.org.", len(nsec))
	}
	if x := nsec[0].(*dns.NSEC).NextDomain; x != "example.org." {
		t.Errorf("Expected NSEC NextDomain %s, got %s", "example.org.", x)
	}
	if x := nsec[0].(*dns.NSEC).TypeBitMap; len(x) != 7 {
		t.Errorf("Expected NSEC bitmap to be %d elements, got %d", 7, x)
	}
	if x := nsec[0].(*dns.NSEC).TypeBitMap; x[6] != dns.TypeCDNSKEY {
		t.Errorf("Expected NSEC bitmap element 5 to be %d, got %d", dns.TypeCDNSKEY, x[6])
	}
	if x := nsec[0].(*dns.NSEC).TypeBitMap; x[4] != dns.TypeDNSKEY {
		t.Errorf("Expected NSEC bitmap element 4 to be %d, got %d", dns.TypeDNSKEY, x[4])
	}
	dnskey := el.Type(dns.TypeDNSKEY)
	if x := dnskey[0].Header().Ttl; x != 1800 {
		t.Errorf("Expected DNSKEY TTL to be %d, got %d", 1800, x)
	}
	sigs := el.Type(dns.TypeRRSIG)
	for _, s := range sigs {
		if s.(*dns.RRSIG).TypeCovered == dns.TypeDNSKEY {
			if s.(*dns.RRSIG).OrigTtl != dnskey[0].Header().Ttl {
				t.Errorf("Expected RRSIG original TTL to match DNSKEY TTL, but %d != %d", s.(*dns.RRSIG).OrigTtl, dnskey[0].Header().Ttl)
			}
			if s.(*dns.RRSIG).SignerName != dnskey[0].Header().Name {
				t.Errorf("Expected RRSIG signer name to match DNSKEY ownername, but %s != %s", s.(*dns.RRSIG).SignerName, dnskey[0].Header().Name)
			}
		}
	}
}

func TestSignGlue(t *testing.T) {
	input := `sign testdata/db.miek.nl miek.nl {
               key file testdata/Kmiek.nl.+013+59725
               directory testdata
       }`
	c := caddy.NewTestController("dns", input)
	sign, err := parse(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(sign.signers) != 1 {
		t.Fatalf("Expected 1 signer, got %d", len(sign.signers))
	}
	z, err := sign.signers[0].Sign(time.Now().UTC())
	if err != nil {
		t.Error(err)
	}

	e, _ := z.Search("ns2.bla.miek.nl.")
	sigs := e.Type(dns.TypeRRSIG)
	if len(sigs) != 0 {
		t.Errorf("Expected no RRSIG for %s, got %d", "ns2.bla.miek.nl.", len(sigs))
	}
}

func TestSignDS(t *testing.T) {
	input := `sign testdata/db.miek.nl_ns miek.nl {
               key file testdata/Kmiek.nl.+013+59725
               directory testdata
       }`
	c := caddy.NewTestController("dns", input)
	sign, err := parse(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(sign.signers) != 1 {
		t.Fatalf("Expected 1 signer, got %d", len(sign.signers))
	}
	z, err := sign.signers[0].Sign(time.Now().UTC())
	if err != nil {
		t.Error(err)
	}

	// dnssec-signzone outputs this for db.miek.nl_ns:
	//
	// child.miek.nl.	1800	IN	NS	ns.child.miek.nl.
	// child.miek.nl.	1800	IN	DS	34385 13 2 fc7397c77afbccb6742fc....
	// child.miek.nl.	1800	IN	RRSIG	DS 13 3 1800 20191223121229 20191123121229 59725 miek.nl. ZwptLzVVs....
	// child.miek.nl.	14400	IN	NSEC	www.miek.nl. NS DS RRSIG NSEC
	// child.miek.nl.	14400	IN	RRSIG	NSEC 13 3 14400 20191223121229 20191123121229 59725 miek.nl. w+CcA8...

	name := "child.miek.nl."
	e, _ := z.Search(name)
	if x := len(e.Types()); x != 4 { // NS DS NSEC and 2x RRSIG
		t.Errorf("Expected 4 records for %s, got %d", name, x)
	}

	ds := e.Type(dns.TypeDS)
	if len(ds) != 1 {
		t.Errorf("Expected DS for %s, got %d", name, len(ds))
	}
	sigs := e.Type(dns.TypeRRSIG)
	if len(sigs) != 2 {
		t.Errorf("Expected no RRSIG for %s, got %d", name, len(sigs))
	}
	nsec := e.Type(dns.TypeNSEC)
	if x := nsec[0].(*dns.NSEC).NextDomain; x != "www.miek.nl." {
		t.Errorf("Expected no NSEC NextDomain to be %s for %s, got %s", "www.miek.nl.", name, x)
	}
	minttl := z.Apex.SOA.Minttl
	if x := nsec[0].Header().Ttl; x != minttl {
		t.Errorf("Expected no NSEC TTL to be %d for %s, got %d", minttl, "www.miek.nl.", x)
	}
	// print zone on error
	buf := &bytes.Buffer{}
	write(buf, z)
	t.Logf("%s\n", buf)
}
