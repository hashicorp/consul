package dnssec

import (
	"testing"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/test"
)

func TestCacheSet(t *testing.T) {
	fPriv, rmPriv, _ := test.TempFile(t, ".", privKey)
	fPub, rmPub, _ := test.TempFile(t, ".", pubKey)
	defer rmPriv()
	defer rmPub()

	dnskey, err := ParseKeyFile(fPub, fPriv)
	if err != nil {
		t.Fatalf("failed to parse key: %v\n", err)
	}

	m := testMsg()
	state := middleware.State{Req: m}
	k := key(m.Answer) // calculate *before* we add the sig
	d := NewDnssec([]string{"miek.nl."}, []*DNSKEY{dnskey}, nil)
	m = d.Sign(state, "miek.nl.", time.Now().UTC())

	_, ok := d.get(k)
	if !ok {
		t.Errorf("signature was not added to the cache")
	}
}
