package dnssec

import (
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/cache"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"
)

func TestCacheSet(t *testing.T) {
	fPriv, rmPriv, _ := test.TempFile(".", privKey)
	fPub, rmPub, _ := test.TempFile(".", pubKey)
	defer rmPriv()
	defer rmPub()

	dnskey, err := ParseKeyFile(fPub, fPriv)
	if err != nil {
		t.Fatalf("Failed to parse key: %v\n", err)
	}

	c := cache.New(defaultCap)
	m := testMsg()
	state := request.Request{Req: m, Zone: "miek.nl."}
	k := hash(m.Answer) // calculate *before* we add the sig
	d := New([]string{"miek.nl."}, []*DNSKEY{dnskey}, false, nil, c)
	d.Sign(state, time.Now().UTC(), server)

	_, ok := d.get(k, server)
	if !ok {
		t.Errorf("Signature was not added to the cache")
	}
}

func TestCacheNotValidExpired(t *testing.T) {
	fPriv, rmPriv, _ := test.TempFile(".", privKey)
	fPub, rmPub, _ := test.TempFile(".", pubKey)
	defer rmPriv()
	defer rmPub()

	dnskey, err := ParseKeyFile(fPub, fPriv)
	if err != nil {
		t.Fatalf("Failed to parse key: %v\n", err)
	}

	c := cache.New(defaultCap)
	m := testMsg()
	state := request.Request{Req: m, Zone: "miek.nl."}
	k := hash(m.Answer) // calculate *before* we add the sig
	d := New([]string{"miek.nl."}, []*DNSKEY{dnskey}, false, nil, c)
	d.Sign(state, time.Now().UTC().AddDate(0, 0, -9), server)

	_, ok := d.get(k, server)
	if ok {
		t.Errorf("Signature was added to the cache even though not valid")
	}
}

func TestCacheNotValidYet(t *testing.T) {
	fPriv, rmPriv, _ := test.TempFile(".", privKey)
	fPub, rmPub, _ := test.TempFile(".", pubKey)
	defer rmPriv()
	defer rmPub()

	dnskey, err := ParseKeyFile(fPub, fPriv)
	if err != nil {
		t.Fatalf("Failed to parse key: %v\n", err)
	}

	c := cache.New(defaultCap)
	m := testMsg()
	state := request.Request{Req: m, Zone: "miek.nl."}
	k := hash(m.Answer) // calculate *before* we add the sig
	d := New([]string{"miek.nl."}, []*DNSKEY{dnskey}, false, nil, c)
	d.Sign(state, time.Now().UTC().AddDate(0, 0, +9), server)

	_, ok := d.get(k, server)
	if ok {
		t.Errorf("Signature was added to the cache even though not valid yet")
	}
}
