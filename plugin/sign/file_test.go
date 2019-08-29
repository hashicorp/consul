package sign

import (
	"os"
	"testing"

	"github.com/miekg/dns"
)

func TestFileParse(t *testing.T) {
	f, err := os.Open("testdata/db.miek.nl")
	if err != nil {
		t.Fatal(err)
	}
	z, err := Parse(f, "miek.nl.", "testdata/db.miek.nl")
	if err != nil {
		t.Fatal(err)
	}
	s := &Signer{
		directory:  ".",
		signedfile: "db.miek.nl.test",
	}

	s.write(z)
	defer os.Remove("db.miek.nl.test")

	f, err = os.Open("db.miek.nl.test")
	if err != nil {
		t.Fatal(err)
	}
	z, err = Parse(f, "miek.nl.", "db.miek.nl.test")
	if err != nil {
		t.Fatal(err)
	}
	if x := z.Apex.SOA.Header().Name; x != "miek.nl." {
		t.Errorf("Expected SOA name to be %s, got %s", x, "miek.nl.")
	}
	apex, _ := z.Search("miek.nl.")
	key := apex.Type(dns.TypeDNSKEY)
	if key != nil {
		t.Errorf("Expected no DNSKEYs, but got %d", len(key))
	}
}
