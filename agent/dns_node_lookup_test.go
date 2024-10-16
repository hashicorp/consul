// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
)

func TestDNS_NodeLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		TaggedAddresses: map[string]string{
			"wan": "127.0.0.2",
		},
		NodeMeta: map[string]string{
			"key": "value",
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)
	require.Len(t, in.Answer, 2)
	require.Len(t, in.Extra, 0)

	aRec, ok := in.Answer[0].(*dns.A)
	require.True(t, ok, "First answer is not an A record")
	require.Equal(t, "127.0.0.1", aRec.A.String())
	require.Equal(t, uint32(0), aRec.Hdr.Ttl)

	txt, ok := in.Answer[1].(*dns.TXT)
	require.True(t, ok, "Second answer is not a TXT record")
	require.Len(t, txt.Txt, 1)
	require.Equal(t, "key=value", txt.Txt[0])

	// Re-do the query, but only for an A RR

	m = new(dns.Msg)
	m.SetQuestion("foo.node.consul.", dns.TypeA)

	c = new(dns.Client)
	in, _, err = c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)
	require.Len(t, in.Answer, 1)
	require.Len(t, in.Extra, 1)

	aRec, ok = in.Answer[0].(*dns.A)
	require.True(t, ok, "Answer is not an A record")
	require.Equal(t, "127.0.0.1", aRec.A.String())
	require.Equal(t, uint32(0), aRec.Hdr.Ttl)

	txt, ok = in.Extra[0].(*dns.TXT)
	require.True(t, ok, "Extra record is not a TXT record")
	require.Len(t, txt.Txt, 1)
	require.Equal(t, "key=value", txt.Txt[0])

	// Re-do the query, but specify the DC
	m = new(dns.Msg)
	m.SetQuestion("foo.node.dc1.consul.", dns.TypeANY)

	c = new(dns.Client)
	in, _, err = c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)
	require.Len(t, in.Answer, 2)
	require.Len(t, in.Extra, 0)

	aRec, ok = in.Answer[0].(*dns.A)
	require.True(t, ok, "First answer is not an A record")
	require.Equal(t, "127.0.0.1", aRec.A.String())
	require.Equal(t, uint32(0), aRec.Hdr.Ttl)

	_, ok = in.Answer[1].(*dns.TXT)
	require.True(t, ok, "Second answer is not a TXT record")

	// lookup a non-existing node, we should receive a SOA
	m = new(dns.Msg)
	m.SetQuestion("nofoo.node.dc1.consul.", dns.TypeANY)

	c = new(dns.Client)
	in, _, err = c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)
	require.Len(t, in.Ns, 1)
	soaRec, ok := in.Ns[0].(*dns.SOA)
	require.True(t, ok, "NS RR is not a SOA record")
	require.Equal(t, uint32(0), soaRec.Hdr.Ttl)
}

func TestDNS_NodeLookup_CaseInsensitive(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "Foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("fOO.node.dc1.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("empty lookup: %#v", in)
	}
}

// V2 DNS does not support node names with a period. This will be deprecated.
func TestDNS_NodeLookup_PeriodName(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node with period in name
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo.bar",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.bar.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	aRec, ok := in.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_NodeLookup_AAAA(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "::4242:4242",
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeAAAA)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	aRec, ok := in.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.AAAA.String() != "::4242:4242" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.Hdr.Ttl != 0 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_NodeLookup_CNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
			dnsTXT("google.com", []string{"my_txt_value"}),
		},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		recursors = ["`+recursor.Addr+`"]
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "google",
		Address:    "www.google.com",
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("google.node.consul.", dns.TypeANY)
	m.SetEdns0(8192, true)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.CNAME{
			Hdr:    dns.RR_Header{Name: "google.node.consul.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 0, Rdlength: 0x10},
			Target: "www.google.com.",
		},
		&dns.CNAME{
			Hdr:    dns.RR_Header{Name: "www.google.com.", Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Rdlength: 0x2},
			Target: "google.com.",
		},
		&dns.A{
			Hdr: dns.RR_Header{Name: "google.com.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x1, 0x2, 0x3, 0x4}, // 1.2.3.4
		},
		&dns.TXT{
			Hdr: dns.RR_Header{Name: "google.com.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Rdlength: 0xd},
			Txt: []string{"my_txt_value"},
		},
	}
	require.Equal(t, wantAnswer, in.Answer)
}

func TestDNS_NodeLookup_TXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "google",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"rfc1035-00": "value0",
			"key0":       "value1",
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("google.node.consul.", dns.TypeTXT)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have the 1 TXT record reply
	if len(in.Answer) != 2 {
		t.Fatalf("Bad: %#v", in)
	}

	txtRec, ok := in.Answer[0].(*dns.TXT)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if len(txtRec.Txt) != 1 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if txtRec.Txt[0] != "value0" && txtRec.Txt[0] != "key0=value1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_NodeLookup_TXT_DontSuppress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = false } `)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "google",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"rfc1035-00": "value0",
			"key0":       "value1",
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("google.node.consul.", dns.TypeTXT)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have the 1 TXT record reply
	if len(in.Answer) != 2 {
		t.Fatalf("Bad: %#v", in)
	}

	txtRec, ok := in.Answer[0].(*dns.TXT)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if len(txtRec.Txt) != 1 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if txtRec.Txt[0] != "value0" && txtRec.Txt[0] != "key0=value1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}

func TestDNS_NodeLookup_ANY(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
		&dns.TXT{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Rdlength: 0xa},
			Txt: []string{"key=value"},
		},
	}
	require.Equal(t, wantAnswer, in.Answer)
}

func TestDNS_NodeLookup_ANY_DontSuppressTXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = false } `)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
		&dns.TXT{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Rdlength: 0xa},
			Txt: []string{"key=value"},
		},
	}
	require.Equal(t, wantAnswer, in.Answer)
}

func TestDNS_NodeLookup_A_SuppressTXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = false } `)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
	}

	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	m := new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeA)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	wantAnswer := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
	}
	require.Equal(t, wantAnswer, in.Answer)

	// ensure TXT RR suppression
	require.Len(t, in.Extra, 0)
}

func TestDNS_NodeLookup_TTL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	recursor := makeRecursor(t, dns.Msg{
		Answer: []dns.RR{
			dnsCNAME("www.google.com", "google.com"),
			dnsA("google.com", "1.2.3.4"),
		},
	})
	defer recursor.Shutdown()

	a := NewTestAgent(t, `
		recursors = ["`+recursor.Addr+`"]
		dns_config {
			node_ttl = "10s"
			allow_stale = true
			max_stale = "1s"
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("foo.node.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	aRec, ok := in.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	// Register node with IPv6
	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "::4242:4242",
	}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check an IPv6 record
	m = new(dns.Msg)
	m.SetQuestion("bar.node.consul.", dns.TypeANY)

	in, _, err = c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	aaaaRec, ok := in.Answer[0].(*dns.AAAA)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aaaaRec.AAAA.String() != "::4242:4242" {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if aaaaRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}

	// Register node with CNAME
	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "google",
		Address:    "www.google.com",
	}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m = new(dns.Msg)
	m.SetQuestion("google.node.consul.", dns.TypeANY)

	in, _, err = c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have the CNAME record + a few A records
	if len(in.Answer) < 2 {
		t.Fatalf("Bad: %#v", in)
	}

	cnRec, ok := in.Answer[0].(*dns.CNAME)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if cnRec.Target != "www.google.com." {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if cnRec.Hdr.Ttl != 10 {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
}
