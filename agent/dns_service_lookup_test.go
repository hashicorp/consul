// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestDNS_ServiceLookupNoMultiCNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "198.18.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "foo.node.consul",
			},
		}

		var out struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// Register a second node node with the same service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "198.18.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "bar.node.consul",
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	// expect a CNAME and an A RR
	require.Len(t, in.Answer, 2)
	require.IsType(t, &dns.CNAME{}, in.Answer[0])
	require.IsType(t, &dns.A{}, in.Answer[1])
}

func TestDNS_ServiceLookupPreferNoCNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "198.18.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "198.18.0.1",
			},
		}

		var out struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// Register a second node node with the same service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "198.18.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "bar.node.consul",
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	// expect an A RR
	require.Len(t, in.Answer, 1)
	aRec, ok := in.Answer[0].(*dns.A)
	require.Truef(t, ok, "Not an A RR")

	require.Equal(t, "db.service.consul.", aRec.Hdr.Name)
	require.Equal(t, "198.18.0.1", aRec.A.String())
}

func TestDNS_ServiceLookupMultiAddrNoCNAME(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "198.18.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "198.18.0.1",
			},
		}

		var out struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// Register a second node node with the same service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "198.18.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "bar.node.consul",
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register a second node node with the same service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "198.18.0.3",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
				Address: "198.18.0.3",
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	// expect two A RRs
	require.Len(t, in.Answer, 2)
	require.IsType(t, &dns.A{}, in.Answer[0])
	require.Equal(t, "db.service.consul.", in.Answer[0].Header().Name)
	isOneOfTheseIPs := func(ip net.IP) bool {
		if ip.Equal(net.ParseIP("198.18.0.1")) || ip.Equal(net.ParseIP("198.18.0.3")) {
			return true
		}
		return false
	}
	require.True(t, isOneOfTheseIPs(in.Answer[0].(*dns.A).A))
	require.IsType(t, &dns.A{}, in.Answer[1])
	require.Equal(t, "db.service.consul.", in.Answer[1].Header().Name)
	require.True(t, isOneOfTheseIPs(in.Answer[1].(*dns.A).A))
}

func TestDNS_ServiceLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
				DNS: structs.QueryDNSOptions{
					TTL: "3s",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}

		if strings.Contains(question, "query") {
			// The query should have the TTL associated with the query registration.
			if srvRec.Hdr.Ttl != 3 {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}
			if aRec.Hdr.Ttl != 3 {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
		} else {
			if srvRec.Hdr.Ttl != 0 {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}
			if aRec.Hdr.Ttl != 0 {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
		}

	}

	// Lookup a non-existing service/query, we should receive an SOA.
	questions = []string{
		"nodb.service.consul.",
		"nope.query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Ns) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		soaRec, ok := in.Ns[0].(*dns.SOA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}
		if soaRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Ns[0])
		}
	}
}

// TestDNS_ServiceAddressWithTagLookup tests some specific cases that Nomad would exercise,
// Like registering a service w/o a Node. https://github.com/hashicorp/nomad/blob/1174019676ff3d65b39323eb0c7234fb1e09b80c/command/agent/consul/service_client.go#L1366-L1381
// Errors with this were reported in https://github.com/hashicorp/consul/issues/21325#issuecomment-2166845574
// Also we test that only one tag is valid in the URL.
func TestDNS_ServiceAddressWithTagLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	{
		// This emulates a Nomad service registration.
		// Using an internal RPC for Catalog.Register will not trigger the same condition.
		err := a.Client().Agent().ServiceRegister(&api.AgentServiceRegistration{
			Kind:    api.ServiceKindTypical,
			ID:      "db-1",
			Name:    "db",
			Tags:    []string{"primary"},
			Address: "127.0.0.1",
			Port:    12345,
			Checks:  make([]*api.AgentServiceCheck, 0),
		})
		require.NoError(t, err)
	}

	{
		err := a.Client().Agent().ServiceRegister(&api.AgentServiceRegistration{
			Kind:    api.ServiceKindTypical,
			ID:      "db-2",
			Name:    "db",
			Tags:    []string{"secondary"},
			Address: "127.0.0.2", // The address here has to be different, or the DNS server will dedupe it.
			Port:    12345,
			Checks:  make([]*api.AgentServiceCheck, 0),
		})
		require.NoError(t, err)
	}

	// Query the service using a tag - this also checks that we're filtering correctly
	questions := []string{
		"_db._primary.service.dc1.consul.", // w/ RFC 2782 style syntax
		"primary.db.service.dc1.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		require.NoError(t, err)
		require.Len(t, in.Answer, 1, "Expected only one result in the Answer section")

		srvRec, ok := in.Answer[0].(*dns.SRV)
		require.True(t, ok, "Expected an SRV record in the Answer section")
		require.Equal(t, uint16(12345), srvRec.Port)
		require.Equal(t, "7f000001.addr.dc1.consul.", srvRec.Target)

		aRec, ok := in.Extra[0].(*dns.A)
		require.True(t, ok, "Expected an A record in the Extra section")
		require.Equal(t, "7f000001.addr.dc1.consul.", aRec.Hdr.Name)
		require.Equal(t, "127.0.0.1", aRec.A.String())

		if strings.Contains(question, "query") {
			// The query should have the TTL associated with the query registration.
			require.Equal(t, uint32(3), srvRec.Hdr.Ttl)
			require.Equal(t, uint32(3), aRec.Hdr.Ttl)
		} else {
			require.Equal(t, uint32(0), srvRec.Hdr.Ttl)
			require.Equal(t, uint32(0), aRec.Hdr.Ttl)
		}
	}

	// Multiple tags are not supported in the legacy DNS server
	questions = []string{
		"banana._db._primary.service.dc1.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		require.NoError(t, err)
		require.Len(t, in.Answer, 0, "Expected no results in the Answer section")

		// We combine the tags with a period, which results in NXDOMAIN
		// The reported issue says that v2dns this is returning valid results.
		require.Equal(t, dns.RcodeNameError, in.Rcode)
	}
}

func TestDNS_ServiceLookupWithInternalServiceAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		node_name = "my.test-node"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	// The service is using the consul DNS name as service address
	// which triggers a lookup loop and a subsequent stack overflow
	// crash.
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Address: "db.service.consul",
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Looking up the service should not trigger a loop
	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAnswer := []dns.RR{
		&dns.SRV{
			Hdr:      dns.RR_Header{Name: "db.service.consul.", Rrtype: 0x21, Class: 0x1, Rdlength: 0x1b},
			Priority: 0x1,
			Weight:   0x1,
			Port:     12345,
			Target:   "foo.node.dc1.consul.",
		},
	}
	require.Equal(t, wantAnswer, in.Answer, "answer")
	wantExtra := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "foo.node.dc1.consul.", Rrtype: 0x1, Class: 0x1, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
	}
	require.Equal(t, wantExtra, in.Extra, "extra")
}

func TestDNS_ConnectServiceLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register
	{
		args := structs.TestRegisterRequestProxy(t)
		args.Address = "127.0.0.55"
		args.Service.Proxy.DestinationServiceName = "db"
		args.Service.Address = ""
		args.Service.Port = 12345
		var out struct{}
		require.Nil(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// Look up the service
	questions := []string{
		"db.connect.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		require.Nil(t, err)
		require.Len(t, in.Answer, 1)

		srvRec, ok := in.Answer[0].(*dns.SRV)
		require.True(t, ok)
		require.Equal(t, uint16(12345), srvRec.Port)
		require.Equal(t, "foo.node.dc1.consul.", srvRec.Target)
		require.Equal(t, uint32(0), srvRec.Hdr.Ttl)

		cnameRec, ok := in.Extra[0].(*dns.A)
		require.True(t, ok)
		require.Equal(t, "foo.node.dc1.consul.", cnameRec.Hdr.Name)
		require.Equal(t, uint32(0), srvRec.Hdr.Ttl)
		require.Equal(t, "127.0.0.55", cnameRec.A.String())
	}

}

func TestDNS_IngressServiceLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register ingress-gateway service
	{
		args := structs.TestRegisterIngressGateway(t)
		var out struct{}
		require.Nil(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// Register db service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Address: "",
				Port:    80,
			},
		}

		var out struct{}
		require.Nil(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// Register proxy-defaults with 'http' protocol
	{
		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry: &structs.ProxyConfigEntry{
				Kind: structs.ProxyDefaults,
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"protocol": "http",
				},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		var out bool
		require.Nil(t, a.RPC(context.Background(), "ConfigEntry.Apply", req, &out))
		require.True(t, out)
	}

	// Register ingress-gateway config entry
	{
		args := &structs.IngressGatewayConfigEntry{
			Name: "ingress-gateway",
			Kind: structs.IngressGateway,
			Listeners: []structs.IngressListener{
				{
					Port:     8888,
					Protocol: "http",
					Services: []structs.IngressService{
						{Name: "db"},
						{Name: "api"},
					},
				},
			},
		}

		req := structs.ConfigEntryRequest{
			Op:         structs.ConfigEntryUpsert,
			Datacenter: "dc1",
			Entry:      args,
		}
		var out bool
		require.Nil(t, a.RPC(context.Background(), "ConfigEntry.Apply", req, &out))
		require.True(t, out)
	}

	// Look up the service
	questions := []string{
		"api.ingress.consul.",
		"api.ingress.dc1.consul.",
		"db.ingress.consul.",
		"db.ingress.dc1.consul.",
	}
	for _, question := range questions {
		t.Run(question, func(t *testing.T) {
			m := new(dns.Msg)
			m.SetQuestion(question, dns.TypeA)

			c := new(dns.Client)
			in, _, err := c.Exchange(m, a.DNSAddr())
			require.Nil(t, err)
			require.Len(t, in.Answer, 1)

			cnameRec, ok := in.Answer[0].(*dns.A)
			require.True(t, ok)
			require.Equal(t, question, cnameRec.Hdr.Name)
			require.Equal(t, uint32(0), cnameRec.Hdr.Ttl)
			require.Equal(t, "127.0.0.1", cnameRec.A.String())
		})
	}
}

func TestDNS_ExternalServiceLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with an external service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "www.google.com",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service
	questions := []string{
		"db.service.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 || len(in.Extra) > 0 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}
}

func TestDNS_ExternalServiceToConsulCNAMELookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		domain = "CONSUL."
		node_name = "test node"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the initial node with a service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "web",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "web",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an external service pointing to the 'web' service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "alias",
			Address:    "web.service.consul",
			Service: &structs.NodeService{
				Service: "alias",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly
	questions := []string{
		"alias.service.consul.",
		"alias.service.CoNsUl.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "web.service.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		if len(in.Extra) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "web.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}

}

func TestDNS_ExternalServiceToConsulCNAMENestedLookup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		node_name = "test-node"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the initial node with a service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "web",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "web",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an external service pointing to the 'web' service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "alias",
			Address:    "web.service.consul",
			Service: &structs.NodeService{
				Service: "alias",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an external service pointing to the 'alias' service
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "alias2",
			Address:    "alias.service.consul",
			Service: &structs.NodeService{
				Service: "alias2",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly
	questions := []string{
		"alias2.service.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "alias.service.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if len(in.Extra) != 2 {
			t.Fatalf("Bad: %#v", in)
		}

		cnameRec, ok := in.Extra[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Name != "alias.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Target != "web.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnameRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}

		aRec, ok := in.Extra[1].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if aRec.Hdr.Name != "web.service.consul." {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddress_A(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "127.0.0.2",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "7f000002.addr.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "7f000002.addr.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.2" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}

}

func TestDNS_AltDomain_ServiceLookup_ServiceAddress_A(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		alt_domain = "test-domain"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "127.0.0.2",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []struct {
		ask        string
		wantDomain string
	}{
		{"db.service.consul.", "consul."},
		{id + ".query.consul.", "consul."},
		{"db.service.test-domain.", "test-domain."},
		{id + ".query.test-domain.", "test-domain."},
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question.ask, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "7f000002.addr.dc1."+question.wantDomain {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "7f000002.addr.dc1."+question.wantDomain {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.2" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddress_SRV(t *testing.T) {
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
		       `)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service whose address isn't an IP.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "www.google.com",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	// Specify prepared query name containing "." to test
	// since that is technically supported (though atypical).
	var id string
	preparedQueryName := "query.name.with.dots"
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: preparedQueryName,
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
		preparedQueryName + ".query.consul.",
		fmt.Sprintf("_%s._tcp.query.consul.", id),
		fmt.Sprintf("_%s._tcp.query.consul.", preparedQueryName),
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		// Should have google CNAME
		cnRec, ok := in.Extra[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if cnRec.Target != "google.com." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}

		// Check we recursively resolve
		aRec, ok := in.Extra[1].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[1])
		}
		if aRec.A.String() != "1.2.3.4" {
			t.Fatalf("Bad: %s", aRec.A.String())
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddressIPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "2607:20:4005:808::200e",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "2607002040050808000000000000200e.addr.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.AAAA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "2607002040050808000000000000200e.addr.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.AAAA.String() != "2607:20:4005:808::200e" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_AltDomain_ServiceLookup_ServiceAddressIPV6(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		alt_domain = "test-domain"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Address: "2607:20:4005:808::200e",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []struct {
		ask  string
		want string
	}{
		{"db.service.consul.", "2607002040050808000000000000200e.addr.dc1.consul."},
		{"db.service.test-domain.", "2607002040050808000000000000200e.addr.dc1.test-domain."},
		{id + ".query.consul.", "2607002040050808000000000000200e.addr.dc1.consul."},
		{id + ".query.test-domain.", "2607002040050808000000000000200e.addr.dc1.test-domain."},
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question.ask, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != question.want {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.AAAA)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != question.want {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.AAAA.String() != "2607:20:4005:808::200e" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_ServiceLookup_WanTranslation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a1 := NewTestAgent(t, `
		datacenter = "dc1"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a1.Shutdown()

	a2 := NewTestAgent(t, `
		datacenter = "dc2"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a2.Shutdown()

	// Join WAN cluster
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	_, err := a2.JoinWAN([]string{addr})
	require.NoError(t, err)
	retry.Run(t, func(r *retry.R) {
		require.Len(r, a1.WANMembers(), 2)
		require.Len(r, a2.WANMembers(), 2)
	})

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc2",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		require.NoError(t, a2.RPC(context.Background(), "PreparedQuery.Apply", args, &id))
	}

	type testCase struct {
		nodeTaggedAddresses    map[string]string
		serviceAddress         string
		serviceTaggedAddresses map[string]structs.ServiceAddress

		dnsAddr string

		expectedPort    uint16
		expectedAddress string
		expectedARRName string
	}

	cases := map[string]testCase{
		"node-addr-from-dc1": {
			dnsAddr:         a1.config.DNSAddrs[0].String(),
			expectedPort:    8080,
			expectedAddress: "127.0.0.1",
			expectedARRName: "foo.node.dc2.consul.",
		},
		"node-wan-from-dc1": {
			dnsAddr: a1.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			expectedPort:    8080,
			expectedAddress: "127.0.0.2",
			expectedARRName: "7f000002.addr.dc2.consul.",
		},
		"service-addr-from-dc1": {
			dnsAddr: a1.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			serviceAddress:  "10.0.1.1",
			expectedPort:    8080,
			expectedAddress: "10.0.1.1",
			expectedARRName: "0a000101.addr.dc2.consul.",
		},
		"service-wan-from-dc1": {
			dnsAddr: a1.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			serviceAddress: "10.0.1.1",
			serviceTaggedAddresses: map[string]structs.ServiceAddress{
				"wan": {
					Address: "198.18.0.1",
					Port:    80,
				},
			},
			expectedPort:    80,
			expectedAddress: "198.18.0.1",
			expectedARRName: "c6120001.addr.dc2.consul.",
		},
		"node-addr-from-dc2": {
			dnsAddr:         a2.config.DNSAddrs[0].String(),
			expectedPort:    8080,
			expectedAddress: "127.0.0.1",
			expectedARRName: "foo.node.dc2.consul.",
		},
		"node-wan-from-dc2": {
			dnsAddr: a2.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			expectedPort:    8080,
			expectedAddress: "127.0.0.1",
			expectedARRName: "foo.node.dc2.consul.",
		},
		"service-addr-from-dc2": {
			dnsAddr: a2.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			serviceAddress:  "10.0.1.1",
			expectedPort:    8080,
			expectedAddress: "10.0.1.1",
			expectedARRName: "0a000101.addr.dc2.consul.",
		},
		"service-wan-from-dc2": {
			dnsAddr: a2.config.DNSAddrs[0].String(),
			nodeTaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			serviceAddress: "10.0.1.1",
			serviceTaggedAddresses: map[string]structs.ServiceAddress{
				"wan": {
					Address: "198.18.0.1",
					Port:    80,
				},
			},
			expectedPort:    8080,
			expectedAddress: "10.0.1.1",
			expectedARRName: "0a000101.addr.dc2.consul.",
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {
			// Register a remote node with a service. This is in a retry since we
			// need the datacenter to have a route which takes a little more time
			// beyond the join, and we don't have direct access to the router here.
			retry.Run(t, func(r *retry.R) {
				args := &structs.RegisterRequest{
					Datacenter:      "dc2",
					Node:            "foo",
					Address:         "127.0.0.1",
					TaggedAddresses: tc.nodeTaggedAddresses,
					Service: &structs.NodeService{
						Service:         "db",
						Address:         tc.serviceAddress,
						Port:            8080,
						TaggedAddresses: tc.serviceTaggedAddresses,
					},
				}

				var out struct{}
				require.NoError(r, a2.RPC(context.Background(), "Catalog.Register", args, &out))
			})

			// Look up the SRV record via service and prepared query.
			questions := []string{
				"db.service.dc2.consul.",
				id + ".query.dc2.consul.",
			}
			for _, question := range questions {
				m := new(dns.Msg)
				m.SetQuestion(question, dns.TypeSRV)

				c := new(dns.Client)

				addr := tc.dnsAddr
				in, _, err := c.Exchange(m, addr)
				require.NoError(t, err)
				require.Len(t, in.Answer, 1)
				srvRec, ok := in.Answer[0].(*dns.SRV)
				require.True(t, ok, "Bad: %#v", in.Answer[0])
				require.Equal(t, tc.expectedPort, srvRec.Port)

				aRec, ok := in.Extra[0].(*dns.A)
				require.True(t, ok, "Bad: %#v", in.Extra[0])
				require.Equal(t, tc.expectedARRName, aRec.Hdr.Name)
				require.Equal(t, tc.expectedAddress, aRec.A.String())
			}

			// Also check the A record directly
			for _, question := range questions {
				m := new(dns.Msg)
				m.SetQuestion(question, dns.TypeA)

				c := new(dns.Client)
				addr := tc.dnsAddr
				in, _, err := c.Exchange(m, addr)
				require.NoError(t, err)
				require.Len(t, in.Answer, 1)

				aRec, ok := in.Answer[0].(*dns.A)
				require.True(t, ok, "Bad: %#v", in.Answer[0])
				require.Equal(t, question, aRec.Hdr.Name)
				require.Equal(t, tc.expectedAddress, aRec.A.String())
			}
		})
	}
}

func TestDNS_ServiceLookup_CaseInsensitive(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		name   string
		config string
	}{
		// UDP + EDNS
		{"normal", ""},
		{"cache", `dns_config{ allow_stale=true, max_stale="3h", use_cache=true, "cache_max_age"="3h"}`},
		{"cache-with-streaming", `
			rpc{
				enable_streaming=true
			}
			use_streaming_backend=true
			dns_config{ allow_stale=true, max_stale="3h", use_cache=true, "cache_max_age"="3h"}
		    `},
	}
	for _, tst := range tests {
		t.Run(fmt.Sprintf("A lookup %v", tst.name), func(t *testing.T) {

			a := NewTestAgent(t, tst.config)
			defer a.Shutdown()
			testrpc.WaitForLeader(t, a.RPC, "dc1")

			// Register a node with a service.
			{
				args := &structs.RegisterRequest{
					Datacenter: "dc1",
					Node:       "foo",
					Address:    "127.0.0.1",
					Service: &structs.NodeService{
						Service: "Db",
						Tags:    []string{"Primary"},
						Port:    12345,
					},
				}

				var out struct{}
				if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
					t.Fatalf("err: %v", err)
				}
			}

			// Register an equivalent prepared query, as well as a name.
			var id string
			{
				args := &structs.PreparedQueryRequest{
					Datacenter: "dc1",
					Op:         structs.PreparedQueryCreate,
					Query: &structs.PreparedQuery{
						Name: "somequery",
						Service: structs.ServiceQuery{
							Service: "db",
						},
					},
				}
				if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
					t.Fatalf("err: %v", err)
				}
			}

			// Try some variations to make sure case doesn't matter.
			questions := []string{
				"primary.Db.service.consul.",
				"primary.db.service.consul.",
				"pRIMARY.dB.service.consul.",
				"PRIMARY.dB.service.consul.",
				"db.service.consul.",
				"DB.service.consul.",
				"Db.service.consul.",
				"somequery.query.consul.",
				"SomeQuery.query.consul.",
				"SOMEQUERY.query.consul.",
			}

			for _, question := range questions {
				m := new(dns.Msg)
				m.SetQuestion(question, dns.TypeSRV)

				c := new(dns.Client)
				retry.Run(t, func(r *retry.R) {
					in, _, err := c.Exchange(m, a.DNSAddr())
					if err != nil {
						r.Fatalf("err: %v", err)
					}

					if len(in.Answer) != 1 {
						r.Fatalf("question %v, empty lookup: %#v", question, in)
					}
				})
			}
		})
	}
}

// We have deprecated support for service tags w/ periods
func TestDNS_ServiceLookup_TagPeriod(t *testing.T) {
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
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"v1.primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m1 := new(dns.Msg)
	m1.SetQuestion("v1.primary2.db.service.consul.", dns.TypeSRV)

	c1 := new(dns.Client)
	in, _, err := c1.Exchange(m1, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 0 {
		t.Fatalf("Bad: %#v", in)
	}

	m := new(dns.Msg)
	m.SetQuestion("v1.primary.db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err = c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	srvRec, ok := in.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if srvRec.Port != 12345 {
		t.Fatalf("Bad: %#v", srvRec)
	}
	if srvRec.Target != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", srvRec)
	}

	aRec, ok := in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Name != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
}

// TestDNS_ServiceLookup_ExtraTags tests tag behavior.
func TestDNS_ServiceLookup_ExtraTags(t *testing.T) {
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
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m1 := new(dns.Msg)
	m1.SetQuestion("dummy.primary.db.service.consul.", dns.TypeSRV)

	c1 := new(dns.Client)
	in, _, err := c1.Exchange(m1, a.DNSAddr())
	require.NoError(t, err)
	require.Len(t, in.Answer, 0, "Expected no answer")
	require.Equal(t, dns.RcodeNameError, in.Rcode)
}

func TestDNS_ServiceLookup_PreparedQueryNamePeriod(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register a prepared query with a period in the name.
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "some.query.we.like",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}

		var id string
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	m := new(dns.Msg)
	m.SetQuestion("some.query.we.like.query.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(in.Answer) != 1 {
		t.Fatalf("Bad: %#v", in)
	}

	srvRec, ok := in.Answer[0].(*dns.SRV)
	if !ok {
		t.Fatalf("Bad: %#v", in.Answer[0])
	}
	if srvRec.Port != 12345 {
		t.Fatalf("Bad: %#v", srvRec)
	}
	if srvRec.Target != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", srvRec)
	}

	aRec, ok := in.Extra[0].(*dns.A)
	if !ok {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.Hdr.Name != "foo.node.dc1.consul." {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
	if aRec.A.String() != "127.0.0.1" {
		t.Fatalf("Bad: %#v", in.Extra[0])
	}
}

func TestDNS_ServiceLookup_Dedup(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a single node with multiple instances of a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
				Tags:    []string{"replica"},
				Port:    12345,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db3",
				Service: "db",
				Tags:    []string{"replica"},
				Port:    12346,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query, make sure only
	// one IP is returned.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

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
}

func TestDNS_ServiceLookup_Dedup_SRV(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a single node with multiple instances of a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db2",
				Service: "db",
				Tags:    []string{"replica"},
				Port:    12345,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args = &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				ID:      "db3",
				Service: "db",
				Tags:    []string{"replica"},
				Port:    12346,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query, make sure only
	// one IP is returned and two unique ports are returned.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 2 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 && srvRec.Port != 12346 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}

		srvRec, ok = in.Answer[1].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}
		if srvRec.Port != 12346 && srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Port == in.Answer[0].(*dns.SRV).Port {
			t.Fatalf("should be a different port")
		}
		if srvRec.Target != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_ServiceLookup_FilterCritical(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register nodes with health checks in various states.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID: "serf",
				Name:    "serf",
				Status:  api.HealthCritical,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args2 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID: "serf",
				Name:    "serf",
				Status:  api.HealthCritical,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args2, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args3 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args3, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args4 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args4, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args5 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "quux",
			Address:    "127.0.0.4",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthWarning,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args5, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Only 4 and 5 are not failing, so we should get 2 answers
		if len(in.Answer) != 2 {
			t.Fatalf("Bad: %#v", in)
		}

		ips := make(map[string]bool)
		for _, resp := range in.Answer {
			aRec := resp.(*dns.A)
			ips[aRec.A.String()] = true
		}

		if !ips["127.0.0.3"] {
			t.Fatalf("Bad: %#v should contain 127.0.0.3 (state healthy)", in)
		}
		if !ips["127.0.0.4"] {
			t.Fatalf("Bad: %#v should contain 127.0.0.4 (state warning)", in)
		}
	}
}

func TestDNS_ServiceLookup_OnlyFailing(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register nodes with all health checks in a critical state.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID: "serf",
				Name:    "serf",
				Status:  api.HealthCritical,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args2 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID: "serf",
				Name:    "serf",
				Status:  api.HealthCritical,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args2, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args3 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},
		}
		if err := a.RPC(context.Background(), "Catalog.Register", args3, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "db",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// All 3 are failing, so we should get 0 answers and an NXDOMAIN response
		if len(in.Answer) != 0 {
			t.Fatalf("Bad: %#v", in)
		}

		if in.Rcode != dns.RcodeNameError {
			t.Fatalf("Bad: %#v", in)
		}
	}
}

func TestDNS_ServiceLookup_OnlyPassing(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		dns_config {
			only_passing = true
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register nodes with health checks in various states.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthPassing,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args2 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.2",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthWarning,
			},
		}

		if err := a.RPC(context.Background(), "Catalog.Register", args2, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		args3 := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "baz",
			Address:    "127.0.0.3",
			Service: &structs.NodeService{
				Service: "db",
				Tags:    []string{"primary"},
				Port:    12345,
			},
			Check: &structs.HealthCheck{
				CheckID:   "db",
				Name:      "db",
				ServiceID: "db",
				Status:    api.HealthCritical,
			},
		}

		if err := a.RPC(context.Background(), "Catalog.Register", args3, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service:     "db",
					OnlyPassing: true,
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"db.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Only 1 is passing, so we should only get 1 answer
		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		resp := in.Answer[0]
		aRec := resp.(*dns.A)

		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
	}

	newCfg := *a.Config
	newCfg.DNSOnlyPassing = false
	err := a.reloadConfigInternal(&newCfg)
	require.NoError(t, err)

	// only_passing is now false. we should now get two nodes
	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeANY)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	require.NoError(t, err)

	require.Equal(t, 2, len(in.Answer))
	ips := []string{in.Answer[0].(*dns.A).A.String(), in.Answer[1].(*dns.A).A.String()}
	sort.Strings(ips)
	require.Equal(t, []string{"127.0.0.1", "127.0.0.2"}, ips)
}

func TestDNS_ServiceLookup_Randomize(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a large number of nodes.
	for i := 0; i < generateNumNodes; i++ {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    fmt.Sprintf("127.0.0.%d", i+1),
			Service: &structs.NodeService{
				Service: "web",
				Port:    8000,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "web",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query. Ensure the
	// response is randomized each time.
	questions := []string{
		"web.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		uniques := map[string]struct{}{}
		for i := 0; i < 10; i++ {
			m := new(dns.Msg)
			m.SetQuestion(question, dns.TypeANY)

			c := &dns.Client{Net: "udp"}
			in, _, err := c.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// Response length should be truncated and we should get
			// an A record for each response.
			if len(in.Answer) != defaultNumUDPResponses {
				t.Fatalf("Bad: %#v", len(in.Answer))
			}

			// Collect all the names.
			var names []string
			for _, rec := range in.Answer {
				switch v := rec.(type) {
				case *dns.SRV:
					names = append(names, v.Target)
				case *dns.A:
					names = append(names, v.A.String())
				}
			}
			nameS := strings.Join(names, "|")

			// Tally the results.
			uniques[nameS] = struct{}{}
		}

		// Give some wiggle room. Since the responses are randomized and
		// there is a finite number of combinations, requiring 0
		// duplicates every test run eventually gives us failures.
		if len(uniques) < 2 {
			t.Fatalf("unique response ratio too low: %d/10\n%v", len(uniques), uniques)
		}
	}
}

func TestDNS_ServiceLookup_Truncate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		dns_config {
			enable_truncate = true
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a large number of nodes.
	for i := 0; i < generateNumNodes; i++ {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    fmt.Sprintf("127.0.0.%d", i+1),
			Service: &structs.NodeService{
				Service: "web",
				Port:    8000,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "web",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query. Ensure the
	// response is truncated each time.
	questions := []string{
		"web.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Check for the truncate bit
		if !in.Truncated {
			t.Fatalf("should have truncate bit")
		}
	}
}

func TestDNS_ServiceLookup_LargeResponses(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		dns_config {
			enable_truncate = true
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	longServiceName := "this-is-a-very-very-very-very-very-long-name-for-a-service"

	// Register a lot of nodes.
	for i := 0; i < 4; i++ {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    fmt.Sprintf("127.0.0.%d", i+1),
			Service: &structs.NodeService{
				Service: longServiceName,
				Tags:    []string{"primary"},
				Port:    12345,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: longServiceName,
				Service: structs.ServiceQuery{
					Service: longServiceName,
					Tags:    []string{"primary"},
				},
			},
		}
		var id string
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"_" + longServiceName + "._primary.service.consul.",
		longServiceName + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if !in.Truncated {
			t.Fatalf("should have truncate bit")
		}

		// Make sure the response size is RFC 1035-compliant for UDP messages
		if in.Len() > 512 {
			t.Fatalf("Bad: %d", in.Len())
		}

		// We should only have two answers now
		if len(in.Answer) != 2 {
			t.Fatalf("Bad: %d", len(in.Answer))
		}

		// Make sure the ADDITIONAL section matches the ANSWER section.
		if len(in.Answer) != len(in.Extra) {
			t.Fatalf("Bad: %d vs. %d", len(in.Answer), len(in.Extra))
		}
		for i := 0; i < len(in.Answer); i++ {
			srv, ok := in.Answer[i].(*dns.SRV)
			if !ok {
				t.Fatalf("Bad: %#v", in.Answer[i])
			}

			a, ok := in.Extra[i].(*dns.A)
			if !ok {
				t.Fatalf("Bad: %#v", in.Extra[i])
			}

			if srv.Target != a.Hdr.Name {
				t.Fatalf("Bad: %#v %#v", srv, a)
			}
		}

		// Check for the truncate bit
		if !in.Truncated {
			t.Fatalf("should have truncate bit")
		}
	}
}

func testDNSServiceLookupResponseLimits(questions []string, answerLimit int, qType uint16,
	expectedService, expectedQuery, expectedQueryID int, a *TestAgent) (bool, error) {
	for idx, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, qType)

		c := &dns.Client{Net: "udp"}
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			return false, fmt.Errorf("err: %v", err)
		}

		switch idx {
		case 0:
			if (expectedService > 0 && len(in.Answer) != expectedService) ||
				(expectedService < -1 && len(in.Answer) < lib.AbsInt(expectedService)) {
				return false, fmt.Errorf("%d/%d answers received for type %v for %s, sz:=%d", len(in.Answer), answerLimit, qType, question, in.Len())
			}
		case 1:
			if (expectedQuery > 0 && len(in.Answer) != expectedQuery) ||
				(expectedQuery < -1 && len(in.Answer) < lib.AbsInt(expectedQuery)) {
				return false, fmt.Errorf("%d/%d answers received for type %v for %s, sz:=%d", len(in.Answer), answerLimit, qType, question, in.Len())
			}
		case 2:
			if (expectedQueryID > 0 && len(in.Answer) != expectedQueryID) ||
				(expectedQueryID < -1 && len(in.Answer) < lib.AbsInt(expectedQueryID)) {
				return false, fmt.Errorf("%d/%d answers received for type %v for %s, sz:=%d", len(in.Answer), answerLimit, qType, question, in.Len())
			}
		default:
			panic("abort")
		}
	}

	return true, nil
}

func checkDNSService(
	t *testing.T,
	protocol string,
	questions []string,
	a *TestAgent,
	qType uint16,
	expectedResultsCount int,
	udpSize uint16,
	setEDNS0 bool,
) {
	for _, question := range questions {
		t.Run("question: "+question, func(t *testing.T) {

			m := new(dns.Msg)

			m.SetQuestion(question, qType)
			if setEDNS0 {
				m.SetEdns0(udpSize, true)
			}
			c := &dns.Client{Net: protocol, UDPSize: udpSize}
			in, _, err := c.Exchange(m, a.DNSAddr())
			require.NoError(t, err)

			t.Logf("DNS Response for %+v - %+v", m, in)

			require.Equal(t, expectedResultsCount, len(in.Answer),
				"%d/%d answers received for type %v for %s (%s)", len(in.Answer), expectedResultsCount, qType, question, protocol)
		})
	}
}

func registerServicesAndPreparedQuery(t *testing.T, generateNumNodes int, a *TestAgent, serviceUniquenessKey string) []string {
	choices := perfectlyRandomChoices(generateNumNodes, pctNodesWithIPv6)
	serviceName := fmt.Sprintf("api-tier-%s", serviceUniquenessKey)
	for i := 0; i < generateNumNodes; i++ {
		nodeAddress := fmt.Sprintf("127.0.0.%d", i+1)
		if choices[i] {
			nodeAddress = fmt.Sprintf("fe80::%d", i+1)
		}
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       fmt.Sprintf("foo%d", i),
			Address:    nodeAddress,
			Service: &structs.NodeService{
				Service: serviceName,
				Port:    8080,
			},
		}

		var out struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}
	var preparedQueryID string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: serviceName,
				Service: structs.ServiceQuery{
					Service: serviceName,
				},
			},
		}

		require.NoError(t, a.RPC(context.Background(), "PreparedQuery.Apply", args, &preparedQueryID))
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		fmt.Sprintf("%s.service.consul.", serviceName),
		fmt.Sprintf("%s.query.consul.", serviceName),
		preparedQueryID + ".query.consul.",
	}
	return questions
}

func TestDNS_ServiceLookup_ARecordLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	const (
		UDP = "udp"
		TCP = "tcp"
	)

	type testCase struct {
		protocol            string
		aRecordLimit        int
		expectedAResults    int
		expectedAAAAResults int
		expectedANYResults  int
		expectedSRVResults  int
		numNodesTotal       int
		udpSize             uint16
		setEDNS0            bool
	}

	type aRecordLimit struct {
		name  string
		limit int
	}

	tests := map[string]testCase{
		// UDP + EDNS
		"udp-edns-1":   {UDP, 1, 1, 1, 1, 30, 30, 8192, true},
		"udp-edns-2":   {UDP, 2, 2, 2, 2, 30, 30, 8192, true},
		"udp-edns-3":   {UDP, 3, 3, 3, 3, 30, 30, 8192, true},
		"udp-edns-4":   {UDP, 4, 4, 4, 4, 30, 30, 8192, true},
		"udp-edns-5":   {UDP, 5, 5, 5, 5, 30, 30, 8192, true},
		"udp-edns-6":   {UDP, 6, 6, 6, 6, 30, 30, 8192, true},
		"udp-edns-max": {UDP, 6, 2, 1, 3, 3, 3, 8192, true},
		// All UDP without EDNS and no udpAnswerLimit
		// Size of records is limited by UDP payload
		"udp-1": {UDP, 1, 1, 0, 1, 1, 1, 512, false},
		"udp-2": {UDP, 2, 1, 1, 2, 2, 2, 512, false},
		"udp-3": {UDP, 3, 1, 1, 2, 2, 2, 512, false},
		"udp-4": {UDP, 4, 1, 1, 2, 2, 2, 512, false},
		"udp-5": {UDP, 5, 1, 1, 2, 2, 2, 512, false},
		"udp-6": {UDP, 6, 1, 1, 2, 2, 2, 512, false},
		// Only 3 A and 3 SRV records on 512 bytes
		"udp-max": {UDP, 6, 1, 1, 2, 2, 2, 512, false},

		"tcp-1":   {TCP, 1, 1, 1, 1, 30, 30, 0, false},
		"tcp-2":   {TCP, 2, 2, 2, 2, 30, 30, 0, false},
		"tcp-3":   {TCP, 3, 3, 3, 3, 30, 30, 0, false},
		"tcp-4":   {TCP, 4, 4, 4, 4, 30, 30, 0, false},
		"tcp-5":   {TCP, 5, 5, 5, 5, 30, 30, 0, false},
		"tcp-6":   {TCP, 6, 6, 6, 6, 30, 30, 0, false},
		"tcp-max": {TCP, 6, 1, 1, 2, 2, 2, 0, false},
	}
	for _, recordLimit := range []aRecordLimit{
		{"1", 1},
		{"2", 2},
		{"3", 3},
		{"4", 4},
		{"5", 5},
		{"6", 6},
		{"max", 6},
	} {
		t.Run("record-limit-"+recordLimit.name, func(t *testing.T) {
			a := NewTestAgent(t, `
			node_name = "test-node"
			dns_config {
				a_record_limit = `+fmt.Sprintf("%d", recordLimit.limit)+`
				udp_answer_limit = `+fmt.Sprintf("%d", recordLimit.limit)+`
			}
		`)

			defer a.Shutdown()
			testrpc.WaitForTestAgent(t, a.RPC, "dc1")

			for _, testName := range []string{
				fmt.Sprintf("udp-edns-%s", recordLimit.name),
				fmt.Sprintf("udp-%s", recordLimit.name),
				fmt.Sprintf("tcp-%s", recordLimit.name),
			} {
				t.Run(testName, func(t *testing.T) {
					test := tests[testName]
					questions := registerServicesAndPreparedQuery(t, test.numNodesTotal, a, testName)

					t.Run("A", func(t *testing.T) {
						checkDNSService(t, test.protocol, questions, a, dns.TypeA, test.expectedAResults, test.udpSize, test.setEDNS0)
					})

					t.Run("AAAA", func(t *testing.T) {
						checkDNSService(t, test.protocol, questions, a, dns.TypeAAAA, test.expectedAAAAResults, test.udpSize, test.setEDNS0)
					})

					t.Run("ANY", func(t *testing.T) {
						checkDNSService(t, test.protocol, questions, a, dns.TypeANY, test.expectedANYResults, test.udpSize, test.setEDNS0)
					})

					// No limits but the size of records for SRV records, since not subject to randomization issues
					t.Run("SRV", func(t *testing.T) {
						checkDNSService(t, test.protocol, questions, a, dns.TypeSRV, test.numNodesTotal, test.udpSize, test.setEDNS0)
					})
				})
			}
		})
	}
}

func TestDNS_ServiceLookup_AnswerLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Build a matrix of config parameters (udpAnswerLimit), and the
	// length of the response per query type and question.  Negative
	// values imply the test must return at least the abs(value) number
	// of records in the answer section.  This is required because, for
	// example, on OS-X and Linux, the number of answers returned in a
	// 512B response is different even though both platforms are x86_64
	// and using the same version of Go.
	//
	// TODO(sean@): Why is it not identical everywhere when using the
	// same compiler?
	tests := []struct {
		name                string
		udpAnswerLimit      int
		expectedAService    int
		expectedAQuery      int
		expectedAQueryID    int
		expectedAAAAService int
		expectedAAAAQuery   int
		expectedAAAAQueryID int
		expectedANYService  int
		expectedANYQuery    int
		expectedANYQueryID  int
	}{
		{"0", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{"1", 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{"2", 2, 2, 2, 2, 2, 2, 2, 2, 2, 2},
		{"3", 3, 3, 3, 3, 3, 3, 3, 3, 3, 3},
		{"4", 4, 4, 4, 4, 4, 4, 4, 4, 4, 4},
		{"5", 5, 5, 5, 5, 5, 5, 5, 5, 5, 5},
		{"6", 6, 6, 6, 6, 6, 6, 5, 6, 6, -5},
		{"7", 7, 7, 7, 6, 7, 7, 5, 7, 7, -5},
		{"8", 8, 8, 8, 6, 8, 8, 5, 8, 8, -5},
		{"9", 9, 8, 8, 6, 8, 8, 5, 8, 8, -5},
		{"20", 20, 8, 8, 6, 8, 8, 5, 8, -5, -5},
		{"30", 30, 8, 8, 6, 8, 8, 5, 8, -5, -5},
	}
	for _, test := range tests {
		test := test // capture loop var
		t.Run(fmt.Sprintf("answer-limit-%s", test.name), func(t *testing.T) {
			a := NewTestAgent(t, fmt.Sprintf(`
		node_name = "test-node"
		dns_config {
			udp_answer_limit = %d
		}`, test.udpAnswerLimit))
			defer a.Shutdown()
			testrpc.WaitForTestAgent(t, a.RPC, "dc1")
			questions := registerServicesAndPreparedQuery(t, generateNumNodes, a, test.name)

			t.Run(fmt.Sprintf("A lookup %v", test), func(t *testing.T) {
				ok, err := testDNSServiceLookupResponseLimits(questions, test.udpAnswerLimit, dns.TypeA, test.expectedAService, test.expectedAQuery, test.expectedAQueryID, a)
				if !ok {
					t.Fatalf("Expected service A lookup %s to pass: %v", test.name, err)
				}
			})

			t.Run(fmt.Sprintf("AAAA lookup %v", test), func(t *testing.T) {
				ok, err := testDNSServiceLookupResponseLimits(questions, test.udpAnswerLimit, dns.TypeAAAA, test.expectedAAAAService, test.expectedAAAAQuery, test.expectedAAAAQueryID, a)
				if !ok {
					t.Fatalf("Expected service AAAA lookup %s to pass: %v", test.name, err)
				}
			})

			t.Run(fmt.Sprintf("ANY lookup %v", test), func(t *testing.T) {
				ok, err := testDNSServiceLookupResponseLimits(questions, test.udpAnswerLimit, dns.TypeANY, test.expectedANYService, test.expectedANYQuery, test.expectedANYQueryID, a)
				if !ok {
					t.Fatalf("Expected service ANY lookup %s to pass: %v", test.name, err)
				}
			})
		})
	}
}

func TestDNS_ServiceLookup_CNAME(t *testing.T) {
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
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a name for an address.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "google",
			Address:    "www.google.com",
			Service: &structs.NodeService{
				Service: "search",
				Port:    80,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "search",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"search.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Service CNAME, google CNAME, google A record
		if len(in.Answer) != 3 {
			t.Fatalf("Bad: %#v", in)
		}

		// Should have service CNAME
		cnRec, ok := in.Answer[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if cnRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		// Should have google CNAME
		cnRec, ok = in.Answer[1].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}
		if cnRec.Target != "google.com." {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}

		// Check we recursively resolve
		if _, ok := in.Answer[2].(*dns.A); !ok {
			t.Fatalf("Bad: %#v", in.Answer[2])
		}
	}
}

func TestDNS_ServiceLookup_ServiceAddress_CNAME(t *testing.T) {
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
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a name for an address.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "google",
			Address:    "1.2.3.4",
			Service: &structs.NodeService{
				Service: "search",
				Port:    80,
				Address: "www.google.com",
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Register an equivalent prepared query.
	var id string
	{
		args := &structs.PreparedQueryRequest{
			Datacenter: "dc1",
			Op:         structs.PreparedQueryCreate,
			Query: &structs.PreparedQuery{
				Name: "test",
				Service: structs.ServiceQuery{
					Service: "search",
				},
			},
		}
		if err := a.RPC(context.Background(), "PreparedQuery.Apply", args, &id); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Look up the service directly and via prepared query.
	questions := []string{
		"search.service.consul.",
		id + ".query.consul.",
	}
	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeANY)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Service CNAME, google CNAME, google A record
		if len(in.Answer) != 3 {
			t.Fatalf("Bad: %#v", in)
		}

		// Should have service CNAME
		cnRec, ok := in.Answer[0].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if cnRec.Target != "www.google.com." {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		// Should have google CNAME
		cnRec, ok = in.Answer[1].(*dns.CNAME)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}
		if cnRec.Target != "google.com." {
			t.Fatalf("Bad: %#v", in.Answer[1])
		}

		// Check we recursively resolve
		if _, ok := in.Answer[2].(*dns.A); !ok {
			t.Fatalf("Bad: %#v", in.Answer[2])
		}
	}
}

func TestDNS_ServiceLookup_TTL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `
		dns_config {
			service_ttl = {
				"d*" = "42s"
				"db" = "10s"
				"db*" = "66s"
				"*" = "5s"
			}
		allow_stale = true
		max_stale = "1s"
		}
	`)
	defer a.Shutdown()

	for idx, service := range []string{"db", "dblb", "dk", "api"} {
		nodeName := fmt.Sprintf("foo%d", idx)
		address := fmt.Sprintf("127.0.0.%d", idx)
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       nodeName,
			Address:    address,
			Service: &structs.NodeService{
				Service: service,
				Tags:    []string{"primary"},
				Port:    12345 + idx,
			},
		}

		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	c := new(dns.Client)
	expectResult := func(dnsQuery string, expectedTTL uint32) {
		t.Run(dnsQuery, func(t *testing.T) {
			m := new(dns.Msg)
			m.SetQuestion(dnsQuery, dns.TypeSRV)

			in, _, err := c.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			if len(in.Answer) != 1 {
				t.Fatalf("Bad: %#v, len is %d", in, len(in.Answer))
			}

			srvRec, ok := in.Answer[0].(*dns.SRV)
			if !ok {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}
			if srvRec.Hdr.Ttl != expectedTTL {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}

			aRec, ok := in.Extra[0].(*dns.A)
			if !ok {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
			if aRec.Hdr.Ttl != expectedTTL {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
		})
	}
	// Should have its exact TTL
	expectResult("db.service.consul.", 10)
	// Should match db*
	expectResult("dblb.service.consul.", 66)
	// Should match d*
	expectResult("dk.service.consul.", 42)
	// Should match *
	expectResult("api.service.consul.", 5)
}

func TestDNS_ServiceLookup_SRV_RFC(t *testing.T) {
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
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	questions := []string{
		"_db._primary.service.dc1.consul.",
		"_db._primary.service.consul.",
		"_db._primary.dc1.consul.",
		"_db._primary.consul.",
	}

	for _, question := range questions {
		m := new(dns.Msg)
		m.SetQuestion(question, dns.TypeSRV)

		c := new(dns.Client)
		in, _, err := c.Exchange(m, a.DNSAddr())
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if len(in.Answer) != 1 {
			t.Fatalf("Bad: %#v", in)
		}

		srvRec, ok := in.Answer[0].(*dns.SRV)
		if !ok {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}
		if srvRec.Port != 12345 {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Target != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", srvRec)
		}
		if srvRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Answer[0])
		}

		aRec, ok := in.Extra[0].(*dns.A)
		if !ok {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Name != "foo.node.dc1.consul." {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.A.String() != "127.0.0.1" {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
		if aRec.Hdr.Ttl != 0 {
			t.Fatalf("Bad: %#v", in.Extra[0])
		}
	}
}

func TestDNS_ServiceLookup_SRV_RFC_TCP_Default(t *testing.T) {
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
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	questions := []string{
		"_db._tcp.service.dc1.consul.",
		"_db._tcp.service.consul.",
		"_db._tcp.dc1.consul.",
		"_db._tcp.consul.",
	}

	for _, question := range questions {
		t.Run(question, func(t *testing.T) {
			m := new(dns.Msg)
			m.SetQuestion(question, dns.TypeSRV)

			c := new(dns.Client)
			in, _, err := c.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			if len(in.Answer) != 1 {
				t.Fatalf("Bad: %#v", in)
			}

			srvRec, ok := in.Answer[0].(*dns.SRV)
			if !ok {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}
			if srvRec.Port != 12345 {
				t.Fatalf("Bad: %#v", srvRec)
			}
			if srvRec.Target != "foo.node.dc1.consul." {
				t.Fatalf("Bad: %#v", srvRec)
			}
			if srvRec.Hdr.Ttl != 0 {
				t.Fatalf("Bad: %#v", in.Answer[0])
			}

			aRec, ok := in.Extra[0].(*dns.A)
			if !ok {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
			if aRec.Hdr.Name != "foo.node.dc1.consul." {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
			if aRec.A.String() != "127.0.0.1" {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
			if aRec.Hdr.Ttl != 0 {
				t.Fatalf("Bad: %#v", in.Extra[0])
			}
		})
	}
}

func initDNSToken(t *testing.T, rpc RPC) {
	t.Helper()

	reqToken := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			SecretID:          "279d4735-f8ca-4d48-b5cc-c00a9713bbf8",
			Policies:          nil,
			TemplatedPolicies: []*structs.ACLTemplatedPolicy{{TemplateName: "builtin/dns"}},
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	err := rpc.RPC(context.Background(), "ACL.TokenSet", &reqToken, &structs.ACLToken{})
	require.NoError(t, err)
}

func TestDNS_ServiceLookup_FilterACL(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		token   string
		results int
	}{
		{"root", 1},
		{"anonymous", 0},
		{"dns", 1},
	}
	for _, tt := range tests {
		t.Run("ACLToken == "+tt.token, func(t *testing.T) {
			hcl := `
				primary_datacenter = "dc1"

				acl {
					enabled = true
					default_policy = "deny"
					down_policy = "deny"

					tokens {
						initial_management = "root"
`
			if tt.token == "dns" {
				// Create a UUID for dns token since it doesn't have an alias
				dnsToken := "279d4735-f8ca-4d48-b5cc-c00a9713bbf8"

				hcl = hcl + `
						default = "anonymous"
						dns = "` + dnsToken + `"
`
			} else {
				hcl = hcl + `
						default = "` + tt.token + `"
`
			}

			hcl = hcl + `
					}
				}
			`

			a := NewTestAgent(t, hcl)
			defer a.Shutdown()
			testrpc.WaitForLeader(t, a.RPC, "dc1")

			if tt.token == "dns" {
				initDNSToken(t, a)
			}

			// Register a service
			args := &structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.1",
				Service: &structs.NodeService{
					Service: "foo",
					Port:    12345,
				},
				WriteRequest: structs.WriteRequest{Token: "root"},
			}
			var out struct{}
			if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
				t.Fatalf("err: %v", err)
			}

			// Set up the DNS query
			c := new(dns.Client)
			m := new(dns.Msg)
			m.SetQuestion("foo.service.consul.", dns.TypeA)

			in, _, err := c.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if len(in.Answer) != tt.results {
				t.Fatalf("Bad: %#v", in)
			}
		})
	}
}

func TestDNS_ServiceLookup_MetaTXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = true } `)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAdditional := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.dc1.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
		&dns.TXT{
			Hdr: dns.RR_Header{Name: "bar.node.dc1.consul.", Rrtype: dns.TypeTXT, Class: dns.ClassINET, Rdlength: 0xa},
			Txt: []string{"key=value"},
		},
	}
	require.Equal(t, wantAdditional, in.Extra)
}

func TestDNS_ServiceLookup_SuppressTXT(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, `dns_config = { enable_additional_node_meta_txt = false } `)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register a node with a service.
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"key": "value",
		},
		Service: &structs.NodeService{
			Service: "db",
			Tags:    []string{"primary"},
			Port:    12345,
		},
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	m := new(dns.Msg)
	m.SetQuestion("db.service.consul.", dns.TypeSRV)

	c := new(dns.Client)
	in, _, err := c.Exchange(m, a.DNSAddr())
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	wantAdditional := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "bar.node.dc1.consul.", Rrtype: dns.TypeA, Class: dns.ClassINET, Rdlength: 0x4},
			A:   []byte{0x7f, 0x0, 0x0, 0x1}, // 127.0.0.1
		},
	}
	require.Equal(t, wantAdditional, in.Extra)
}
