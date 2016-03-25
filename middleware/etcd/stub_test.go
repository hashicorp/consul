// +build etcd

package etcd

import "testing"

func TestStubLookup(t *testing.T) {
	// e.updateStubZones()
}

/*
func TestDNSStubForward(t *testing.T) {
	s := newTestServer(t, false)
	defer s.Stop()

	c := new(dns.Client)
	m := new(dns.Msg)

	stubEx := &msg.Service{
		// IP address of a.iana-servers.net.
		Host: "199.43.132.53", Key: "a.example.com.stub.dns.skydns.test.",
	}
	stubBroken := &msg.Service{
		Host: "127.0.0.1", Port: 5454, Key: "b.example.org.stub.dns.skydns.test.",
	}
	stubLoop := &msg.Service{
		Host: "127.0.0.1", Port: Port, Key: "b.example.net.stub.dns.skydns.test.",
	}
	addService(t, s, stubEx.Key, 0, stubEx)
	defer delService(t, s, stubEx.Key)
	addService(t, s, stubBroken.Key, 0, stubBroken)
	defer delService(t, s, stubBroken.Key)
	addService(t, s, stubLoop.Key, 0, stubLoop)
	defer delService(t, s, stubLoop.Key)

	s.UpdateStubZones()

	m.SetQuestion("www.example.com.", dns.TypeA)
	resp, _, err := c.Exchange(m, "127.0.0.1:"+StrPort)
	if err != nil {
		// try twice
		resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(resp.Answer) == 0 || resp.Rcode != dns.RcodeSuccess {
		t.Fatal("answer expected to have A records or rcode not equal to RcodeSuccess")
	}
	// The main diff. here is that we expect the AA bit to be set, because we directly
	// queried the authoritative servers.
	if resp.Authoritative != true {
		t.Fatal("answer expected to have AA bit set")
	}

	// This should fail.
	m.SetQuestion("www.example.org.", dns.TypeA)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if len(resp.Answer) != 0 || resp.Rcode != dns.RcodeServerFailure {
		t.Fatal("answer expected to fail for example.org")
	}

	// This should really fail with a timeout.
	m.SetQuestion("www.example.net.", dns.TypeA)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if err == nil {
		t.Fatal("answer expected to fail for example.net")
	} else {
		t.Logf("succesfully failing %s", err)
	}

	// Packet with EDNS0
	m.SetEdns0(4096, true)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+StrPort)
	if err == nil {
		t.Fatal("answer expected to fail for example.net")
	} else {
		t.Logf("succesfully failing %s", err)
	}

	// Now start another SkyDNS instance on a different port,
	// add a stubservice for it and check if the forwarding is
	// actually working.
	oldStrPort := StrPort

	s1 := newTestServer(t, false)
	defer s1.Stop()
	s1.config.Domain = "skydns.com."

	// Add forwarding IP for internal.skydns.com. Use Port to point to server s.
	stubForward := &msg.Service{
		Host: "127.0.0.1", Port: Port, Key: "b.internal.skydns.com.stub.dns.skydns.test.",
	}
	addService(t, s, stubForward.Key, 0, stubForward)
	defer delService(t, s, stubForward.Key)
	s.UpdateStubZones()

	// Add an answer for this in our "new" server.
	stubReply := &msg.Service{
		Host: "127.1.1.1", Key: "www.internal.skydns.com.",
	}
	addService(t, s1, stubReply.Key, 0, stubReply)
	defer delService(t, s1, stubReply.Key)

	m = new(dns.Msg)
	m.SetQuestion("www.internal.skydns.com.", dns.TypeA)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+oldStrPort)
	if err != nil {
		t.Fatalf("failed to forward %s", err)
	}
	if resp.Answer[0].(*dns.A).A.String() != "127.1.1.1" {
		t.Fatalf("failed to get correct reply")
	}

	// Adding an in baliwick internal domain forward.
	s2 := newTestServer(t, false)
	defer s2.Stop()
	s2.config.Domain = "internal.skydns.net."

	// Add forwarding IP for internal.skydns.net. Use Port to point to server s.
	stubForward1 := &msg.Service{
		Host: "127.0.0.1", Port: Port, Key: "b.internal.skydns.net.stub.dns.skydns.test.",
	}
	addService(t, s, stubForward1.Key, 0, stubForward1)
	defer delService(t, s, stubForward1.Key)
	s.UpdateStubZones()

	// Add an answer for this in our "new" server.
	stubReply1 := &msg.Service{
		Host: "127.10.10.10", Key: "www.internal.skydns.net.",
	}
	addService(t, s2, stubReply1.Key, 0, stubReply1)
	defer delService(t, s2, stubReply1.Key)

	m = new(dns.Msg)
	m.SetQuestion("www.internal.skydns.net.", dns.TypeA)
	resp, _, err = c.Exchange(m, "127.0.0.1:"+oldStrPort)
	if err != nil {
		t.Fatalf("failed to forward %s", err)
	}
	if resp.Answer[0].(*dns.A).A.String() != "127.10.10.10" {
		t.Fatalf("failed to get correct reply")
	}
}
*/
