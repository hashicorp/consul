package dnssec

import (
	"testing"
	"time"

	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/coredns/request"

	"github.com/hashicorp/golang-lru"
)

func TestCacheSet(t *testing.T) {
	fPriv, rmPriv, _ := test.TempFile(".", privKey)
	fPub, rmPub, _ := test.TempFile(".", pubKey)
	defer rmPriv()
	defer rmPub()

	dnskey, err := ParseKeyFile(fPub, fPriv)
	if err != nil {
		t.Fatalf("failed to parse key: %v\n", err)
	}

	cache, _ := lru.New(defaultCap)
	m := testMsg()
	state := request.Request{Req: m}
	k := key(m.Answer) // calculate *before* we add the sig
	d := New([]string{"miek.nl."}, []*DNSKEY{dnskey}, nil, cache)
	m = d.Sign(state, "miek.nl.", time.Now().UTC())

	_, ok := d.get(k)
	if !ok {
		t.Errorf("signature was not added to the cache")
	}
}
