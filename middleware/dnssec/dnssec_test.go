package dnssec

import (
	"testing"
	"time"

	"github.com/miekg/coredns/middleware/test"
	"github.com/miekg/coredns/request"

	"github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

func TestZoneSigning(t *testing.T) {
	d, rm1, rm2 := newDnssec(t, []string{"miek.nl."})
	defer rm1()
	defer rm2()

	m := testMsg()
	state := request.Request{Req: m}

	m = d.Sign(state, "miek.nl.", time.Now().UTC())
	if !section(m.Answer, 1) {
		t.Errorf("answer section should have 1 sig")
	}
	if !section(m.Ns, 1) {
		t.Errorf("authority section should have 1 sig")
	}
}

func TestZoneSigningDouble(t *testing.T) {
	d, rm1, rm2 := newDnssec(t, []string{"miek.nl."})
	defer rm1()
	defer rm2()

	fPriv1, rmPriv1, _ := test.TempFile(".", privKey1)
	fPub1, rmPub1, _ := test.TempFile(".", pubKey1)
	defer rmPriv1()
	defer rmPub1()

	key1, err := ParseKeyFile(fPub1, fPriv1)
	if err != nil {
		t.Fatalf("failed to parse key: %v\n", err)
	}
	d.keys = append(d.keys, key1)

	m := testMsg()
	state := request.Request{Req: m}
	m = d.Sign(state, "miek.nl.", time.Now().UTC())
	if !section(m.Answer, 2) {
		t.Errorf("answer section should have 1 sig")
	}
	if !section(m.Ns, 2) {
		t.Errorf("authority section should have 1 sig")
	}
}

// TestSigningDifferentZone tests if a key for miek.nl and be used for example.org.
func TestSigningDifferentZone(t *testing.T) {
	fPriv, rmPriv, _ := test.TempFile(".", privKey)
	fPub, rmPub, _ := test.TempFile(".", pubKey)
	defer rmPriv()
	defer rmPub()

	key, err := ParseKeyFile(fPub, fPriv)
	if err != nil {
		t.Fatalf("failed to parse key: %v\n", err)
	}

	m := testMsgEx()
	state := request.Request{Req: m}
	cache, _ := lru.New(defaultCap)
	d := New([]string{"example.org."}, []*DNSKEY{key}, nil, cache)
	m = d.Sign(state, "example.org.", time.Now().UTC())
	if !section(m.Answer, 1) {
		t.Errorf("answer section should have 1 sig")
		t.Logf("%+v\n", m)
	}
	if !section(m.Ns, 1) {
		t.Errorf("authority section should have 1 sig")
		t.Logf("%+v\n", m)
	}
}

func TestSigningCname(t *testing.T) {
	d, rm1, rm2 := newDnssec(t, []string{"miek.nl."})
	defer rm1()
	defer rm2()

	m := testMsgCname()
	state := request.Request{Req: m}
	m = d.Sign(state, "miek.nl.", time.Now().UTC())
	if !section(m.Answer, 1) {
		t.Errorf("answer section should have 1 sig")
	}
}

func TestZoneSigningDelegation(t *testing.T) {
	d, rm1, rm2 := newDnssec(t, []string{"miek.nl."})
	defer rm1()
	defer rm2()

	m := testDelegationMsg()
	state := request.Request{Req: m}
	m = d.Sign(state, "miek.nl.", time.Now().UTC())
	if !section(m.Ns, 0) {
		t.Errorf("authority section should have 0 sig")
		t.Logf("%v\n", m)
	}
	if !section(m.Extra, 0) {
		t.Errorf("answer section should have 0 sig")
		t.Logf("%v\n", m)
	}
}

func section(rss []dns.RR, nrSigs int) bool {
	i := 0
	for _, r := range rss {
		if r.Header().Rrtype == dns.TypeRRSIG {
			i++
		}
	}
	return nrSigs == i
}

func testMsg() *dns.Msg {
	// don't care about the message header
	return &dns.Msg{
		Answer: []dns.RR{test.MX("miek.nl.	1703	IN	MX	1 aspmx.l.google.com.")},
		Ns: []dns.RR{test.NS("miek.nl.	1703	IN	NS	omval.tednet.nl.")},
	}
}
func testMsgEx() *dns.Msg {
	return &dns.Msg{
		Answer: []dns.RR{test.MX("example.org.	1703	IN	MX	1 aspmx.l.google.com.")},
		Ns: []dns.RR{test.NS("example.org.	1703	IN	NS	omval.tednet.nl.")},
	}
}

func testMsgCname() *dns.Msg {
	return &dns.Msg{
		Answer: []dns.RR{test.CNAME("www.miek.nl.	1800	IN	CNAME	a.miek.nl.")},
	}
}

func testDelegationMsg() *dns.Msg {
	return &dns.Msg{
		Ns: []dns.RR{
			test.NS("miek.nl.	3600	IN	NS	linode.atoom.net."),
			test.NS("miek.nl.	3600	IN	NS	ns-ext.nlnetlabs.nl."),
			test.NS("miek.nl.	3600	IN	NS	omval.tednet.nl."),
		},
		Extra: []dns.RR{
			test.A("omval.tednet.nl.	3600	IN	A	185.49.141.42"),
			test.AAAA("omval.tednet.nl.	3600	IN	AAAA	2a04:b900:0:100::42"),
		},
	}
}

func newDnssec(t *testing.T, zones []string) (Dnssec, func(), func()) {
	k, rm1, rm2 := newKey(t)
	cache, _ := lru.New(defaultCap)
	d := New(zones, []*DNSKEY{k}, nil, cache)
	return d, rm1, rm2
}

func newKey(t *testing.T) (*DNSKEY, func(), func()) {
	fPriv, rmPriv, _ := test.TempFile(".", privKey)
	fPub, rmPub, _ := test.TempFile(".", pubKey)

	key, err := ParseKeyFile(fPub, fPriv)
	if err != nil {
		t.Fatalf("failed to parse key: %v\n", err)
	}
	return key, rmPriv, rmPub
}

const (
	pubKey  = `miek.nl. IN DNSKEY 257 3 13 0J8u0XJ9GNGFEBXuAmLu04taHG4BXPP3gwhetiOUMnGA+x09nqzgF5IY OyjWB7N3rXqQbnOSILhH1hnuyh7mmA==`
	privKey = `Private-key-format: v1.3
Algorithm: 13 (ECDSAP256SHA256)
PrivateKey: /4BZk8AFvyW5hL3cOLSVxIp1RTqHSAEloWUxj86p3gs=
Created: 20160423195532
Publish: 20160423195532
Activate: 20160423195532
`
	pubKey1  = `example.org. IN DNSKEY 257 3 13 tVRWNSGpHZbCi7Pr7OmbADVUO3MxJ0Lb8Lk3o/HBHqCxf5K/J50lFqRa 98lkdAIiFOVRy8LyMvjwmxZKwB5MNw==`
	privKey1 = `Private-key-format: v1.3
Algorithm: 13 (ECDSAP256SHA256)
PrivateKey: i8j4OfDGT8CQt24SDwLz2hg9yx4qKOEOh1LvbAuSp1c=
Created: 20160423211746
Publish: 20160423211746
Activate: 20160423211746
`
)
