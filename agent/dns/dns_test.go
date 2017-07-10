package dns

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/hashicorp/consul/agent/config"
	mdns "github.com/miekg/dns"
)

const (
	configUDPAnswerLimit   = 4
	defaultNumUDPResponses = 3
	testUDPTruncateLimit   = 8

	pctNodesWithIPv6 = 0.5

	// generateNumNodes is the upper bounds for the number of hosts used
	// in testing below.  Generate an arbitrarily large number of hosts.
	generateNumNodes = testUDPTruncateLimit * defaultNumUDPResponses * configUDPAnswerLimit
)

func TestRecursorAddr(t *testing.T) {
	t.Parallel()
	addr, err := RecursorAddr("8.8.8.8")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if addr != "8.8.8.8:53" {
		t.Fatalf("bad: %v", addr)
	}
}

func TestDNS_trimUDPResponse_NoTrim(t *testing.T) {
	t.Parallel()
	req := &mdns.Msg{}
	resp := &mdns.Msg{
		Answer: []mdns.RR{
			&mdns.SRV{
				Hdr: mdns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: mdns.TypeSRV,
					Class:  mdns.ClassINET,
				},
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
		},
		Extra: []mdns.RR{
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
		},
	}

	cfg := config.DefaultConfig()
	if trimmed := trimUDPResponse(req, resp, cfg.DNSConfig.UDPAnswerLimit); trimmed {
		t.Fatalf("Bad %#v", *resp)
	}

	expected := &mdns.Msg{
		Answer: []mdns.RR{
			&mdns.SRV{
				Hdr: mdns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: mdns.TypeSRV,
					Class:  mdns.ClassINET,
				},
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
		},
		Extra: []mdns.RR{
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
		},
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("Bad %#v vs. %#v", *resp, *expected)
	}
}

func TestDNS_trimUDPResponse_TrimLimit(t *testing.T) {
	t.Parallel()
	cfg := &config.DefaultConfig().DNSConfig

	req, resp, expected := &mdns.Msg{}, &mdns.Msg{}, &mdns.Msg{}
	for i := 0; i < cfg.UDPAnswerLimit+1; i++ {
		target := fmt.Sprintf("ip-10-0-1-%d.node.dc1.consul.", 185+i)
		srv := &mdns.SRV{
			Hdr: mdns.RR_Header{
				Name:   "redis-cache-redis.service.consul.",
				Rrtype: mdns.TypeSRV,
				Class:  mdns.ClassINET,
			},
			Target: target,
		}
		a := &mdns.A{
			Hdr: mdns.RR_Header{
				Name:   target,
				Rrtype: mdns.TypeA,
				Class:  mdns.ClassINET,
			},
			A: net.ParseIP(fmt.Sprintf("10.0.1.%d", 185+i)),
		}

		resp.Answer = append(resp.Answer, srv)
		resp.Extra = append(resp.Extra, a)
		if i < cfg.UDPAnswerLimit {
			expected.Answer = append(expected.Answer, srv)
			expected.Extra = append(expected.Extra, a)
		}
	}

	if trimmed := trimUDPResponse(req, resp, cfg.UDPAnswerLimit); !trimmed {
		t.Fatalf("Bad %#v", *resp)
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("Bad %#v vs. %#v", *resp, *expected)
	}
}

func TestDNS_trimUDPResponse_TrimSize(t *testing.T) {
	t.Parallel()
	cfg := &config.DefaultConfig().DNSConfig

	req, resp := &mdns.Msg{}, &mdns.Msg{}
	for i := 0; i < 100; i++ {
		target := fmt.Sprintf("ip-10-0-1-%d.node.dc1.consul.", 185+i)
		srv := &mdns.SRV{
			Hdr: mdns.RR_Header{
				Name:   "redis-cache-redis.service.consul.",
				Rrtype: mdns.TypeSRV,
				Class:  mdns.ClassINET,
			},
			Target: target,
		}
		a := &mdns.A{
			Hdr: mdns.RR_Header{
				Name:   target,
				Rrtype: mdns.TypeA,
				Class:  mdns.ClassINET,
			},
			A: net.ParseIP(fmt.Sprintf("10.0.1.%d", 185+i)),
		}

		resp.Answer = append(resp.Answer, srv)
		resp.Extra = append(resp.Extra, a)
	}

	// We don't know the exact trim, but we know the resulting answer
	// data should match its extra data.
	if trimmed := trimUDPResponse(req, resp, cfg.UDPAnswerLimit); !trimmed {
		t.Fatalf("Bad %#v", *resp)
	}
	if len(resp.Answer) == 0 || len(resp.Answer) != len(resp.Extra) {
		t.Fatalf("Bad %#v", *resp)
	}
	for i := range resp.Answer {
		srv, ok := resp.Answer[i].(*mdns.SRV)
		if !ok {
			t.Fatalf("should be SRV")
		}

		a, ok := resp.Extra[i].(*mdns.A)
		if !ok {
			t.Fatalf("should be A")
		}

		if srv.Target != a.Header().Name {
			t.Fatalf("Bad %#v vs. %#v", *srv, *a)
		}
	}
}

func TestDNS_trimUDPResponse_TrimSizeEDNS(t *testing.T) {
	t.Parallel()
	cfg := &config.DefaultConfig().DNSConfig

	req, resp := &mdns.Msg{}, &mdns.Msg{}

	for i := 0; i < 100; i++ {
		target := fmt.Sprintf("ip-10-0-1-%d.node.dc1.consul.", 150+i)
		srv := &mdns.SRV{
			Hdr: mdns.RR_Header{
				Name:   "redis-cache-redis.service.consul.",
				Rrtype: mdns.TypeSRV,
				Class:  mdns.ClassINET,
			},
			Target: target,
		}
		a := &mdns.A{
			Hdr: mdns.RR_Header{
				Name:   target,
				Rrtype: mdns.TypeA,
				Class:  mdns.ClassINET,
			},
			A: net.ParseIP(fmt.Sprintf("10.0.1.%d", 150+i)),
		}

		resp.Answer = append(resp.Answer, srv)
		resp.Extra = append(resp.Extra, a)
	}

	// Copy over to a new slice since we are trimming both.
	reqEDNS, respEDNS := &mdns.Msg{}, &mdns.Msg{}
	reqEDNS.SetEdns0(2048, true)
	respEDNS.Answer = append(respEDNS.Answer, resp.Answer...)
	respEDNS.Extra = append(respEDNS.Extra, resp.Extra...)

	// Trim each response
	if trimmed := trimUDPResponse(req, resp, cfg.UDPAnswerLimit); !trimmed {
		t.Errorf("expected response to be trimmed: %#v", resp)
	}
	if trimmed := trimUDPResponse(reqEDNS, respEDNS, cfg.UDPAnswerLimit); !trimmed {
		t.Errorf("expected edns to be trimmed: %#v", resp)
	}

	// Check answer lengths
	if len(resp.Answer) == 0 || len(resp.Answer) != len(resp.Extra) {
		t.Errorf("bad response answer length: %#v", resp)
	}
	if len(respEDNS.Answer) == 0 || len(respEDNS.Answer) != len(respEDNS.Extra) {
		t.Errorf("bad edns answer length: %#v", resp)
	}

	// Due to the compression, we can't check exact equality of sizes, but we can
	// make two requests and ensure that the edns one returns a larger payload
	// than the non-edns0 one.
	if len(resp.Answer) >= len(respEDNS.Answer) {
		t.Errorf("expected edns have larger answer: %#v\n%#v", resp, respEDNS)
	}
	if len(resp.Extra) >= len(respEDNS.Extra) {
		t.Errorf("expected edns have larger extra: %#v\n%#v", resp, respEDNS)
	}

	// Verify that the things point where they should
	for i := range resp.Answer {
		srv, ok := resp.Answer[i].(*mdns.SRV)
		if !ok {
			t.Errorf("%d should be an SRV", i)
		}

		a, ok := resp.Extra[i].(*mdns.A)
		if !ok {
			t.Errorf("%d should be an A", i)
		}

		if srv.Target != a.Header().Name {
			t.Errorf("%d: bad %#v vs. %#v", i, srv, a)
		}
	}
}

func TestDNS_syncExtra(t *testing.T) {
	t.Parallel()
	resp := &mdns.Msg{
		Answer: []mdns.RR{
			// These two are on the same host so the redundant extra
			// records should get deduplicated.
			&mdns.SRV{
				Hdr: mdns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: mdns.TypeSRV,
					Class:  mdns.ClassINET,
				},
				Port:   1001,
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
			&mdns.SRV{
				Hdr: mdns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: mdns.TypeSRV,
					Class:  mdns.ClassINET,
				},
				Port:   1002,
				Target: "ip-10-0-1-185.node.dc1.consul.",
			},
			// This one isn't in the Consul domain so it will get a
			// CNAME and then an A record from the recursor.
			&mdns.SRV{
				Hdr: mdns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: mdns.TypeSRV,
					Class:  mdns.ClassINET,
				},
				Port:   1003,
				Target: "demo.consul.io.",
			},
			// This one isn't in the Consul domain and it will get
			// a CNAME and A record from a recursor that alters the
			// case of the name. This proves we look up in the index
			// in a case-insensitive way.
			&mdns.SRV{
				Hdr: mdns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: mdns.TypeSRV,
					Class:  mdns.ClassINET,
				},
				Port:   1001,
				Target: "insensitive.consul.io.",
			},
			// This is also a CNAME, but it'll be set up to loop to
			// make sure we don't crash.
			&mdns.SRV{
				Hdr: mdns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: mdns.TypeSRV,
					Class:  mdns.ClassINET,
				},
				Port:   1001,
				Target: "deadly.consul.io.",
			},
			// This is also a CNAME, but it won't have another record.
			&mdns.SRV{
				Hdr: mdns.RR_Header{
					Name:   "redis-cache-redis.service.consul.",
					Rrtype: mdns.TypeSRV,
					Class:  mdns.ClassINET,
				},
				Port:   1001,
				Target: "nope.consul.io.",
			},
		},
		Extra: []mdns.RR{
			// These should get deduplicated.
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			// This is a normal CNAME followed by an A record but we
			// have flipped the order. The algorithm should emit them
			// in the opposite order.
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "fakeserver.consul.io.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&mdns.CNAME{
				Hdr: mdns.RR_Header{
					Name:   "demo.consul.io.",
					Rrtype: mdns.TypeCNAME,
					Class:  mdns.ClassINET,
				},
				Target: "fakeserver.consul.io.",
			},
			// These differ in case to test case insensitivity.
			&mdns.CNAME{
				Hdr: mdns.RR_Header{
					Name:   "INSENSITIVE.CONSUL.IO.",
					Rrtype: mdns.TypeCNAME,
					Class:  mdns.ClassINET,
				},
				Target: "Another.Server.Com.",
			},
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "another.server.com.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			// This doesn't appear in the answer, so should get
			// dropped.
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "ip-10-0-1-186.node.dc1.consul.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("10.0.1.186"),
			},
			// These two test edge cases with CNAME handling.
			&mdns.CNAME{
				Hdr: mdns.RR_Header{
					Name:   "deadly.consul.io.",
					Rrtype: mdns.TypeCNAME,
					Class:  mdns.ClassINET,
				},
				Target: "deadly.consul.io.",
			},
			&mdns.CNAME{
				Hdr: mdns.RR_Header{
					Name:   "nope.consul.io.",
					Rrtype: mdns.TypeCNAME,
					Class:  mdns.ClassINET,
				},
				Target: "notthere.consul.io.",
			},
		},
	}

	index := make(map[string]mdns.RR)
	indexRRs(resp.Extra, index)
	syncExtra(index, resp)

	expected := &mdns.Msg{
		Answer: resp.Answer,
		Extra: []mdns.RR{
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "ip-10-0-1-185.node.dc1.consul.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("10.0.1.185"),
			},
			&mdns.CNAME{
				Hdr: mdns.RR_Header{
					Name:   "demo.consul.io.",
					Rrtype: mdns.TypeCNAME,
					Class:  mdns.ClassINET,
				},
				Target: "fakeserver.consul.io.",
			},
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "fakeserver.consul.io.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&mdns.CNAME{
				Hdr: mdns.RR_Header{
					Name:   "INSENSITIVE.CONSUL.IO.",
					Rrtype: mdns.TypeCNAME,
					Class:  mdns.ClassINET,
				},
				Target: "Another.Server.Com.",
			},
			&mdns.A{
				Hdr: mdns.RR_Header{
					Name:   "another.server.com.",
					Rrtype: mdns.TypeA,
					Class:  mdns.ClassINET,
				},
				A: net.ParseIP("127.0.0.1"),
			},
			&mdns.CNAME{
				Hdr: mdns.RR_Header{
					Name:   "deadly.consul.io.",
					Rrtype: mdns.TypeCNAME,
					Class:  mdns.ClassINET,
				},
				Target: "deadly.consul.io.",
			},
			&mdns.CNAME{
				Hdr: mdns.RR_Header{
					Name:   "nope.consul.io.",
					Rrtype: mdns.TypeCNAME,
					Class:  mdns.ClassINET,
				},
				Target: "notthere.consul.io.",
			},
		},
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("Bad %#v vs. %#v", *resp, *expected)
	}
}

func TestDNS_Compression_trimUDPResponse(t *testing.T) {
	t.Parallel()
	cfg := &config.DefaultConfig().DNSConfig

	req, m := mdns.Msg{}, mdns.Msg{}
	trimUDPResponse(&req, &m, cfg.UDPAnswerLimit)
	if m.Compress {
		t.Fatalf("compression should be off")
	}

	// The trim function temporarily turns off compression, so we need to
	// make sure the setting gets restored properly.
	m.Compress = true
	trimUDPResponse(&req, &m, cfg.UDPAnswerLimit)
	if !m.Compress {
		t.Fatalf("compression should be on")
	}
}
