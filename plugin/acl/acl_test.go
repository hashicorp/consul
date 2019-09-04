package acl

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/test"

	"github.com/caddyserver/caddy"
	"github.com/miekg/dns"
)

type testResponseWriter struct {
	test.ResponseWriter
	Rcode int
}

func (t *testResponseWriter) setRemoteIP(ip string) {
	t.RemoteIP = ip
}

// WriteMsg implement dns.ResponseWriter interface.
func (t *testResponseWriter) WriteMsg(m *dns.Msg) error {
	t.Rcode = m.Rcode
	return nil
}

func NewTestControllerWithZones(input string, zones []string) *caddy.Controller {
	ctr := caddy.NewTestController("dns", input)
	for _, zone := range zones {
		ctr.ServerBlockKeys = append(ctr.ServerBlockKeys, zone)
	}
	return ctr
}

func TestACLServeDNS(t *testing.T) {
	type args struct {
		domain   string
		sourceIP string
		qtype    uint16
	}
	tests := []struct {
		name      string
		config    string
		zones     []string
		args      args
		wantRcode int
		wantErr   bool
	}{
		// IPv4 tests.
		{
			"Blacklist 1 BLOCKED",
			`acl example.org {
				block type A net 192.168.0.0/16
			}`,
			[]string{},
			args{
				"www.example.org.",
				"192.168.0.2",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Blacklist 1 ALLOWED",
			`acl example.org {
				block type A net 192.168.0.0/16
			}`,
			[]string{},
			args{
				"www.example.org.",
				"192.167.0.2",
				dns.TypeA,
			},
			dns.RcodeSuccess,
			false,
		},
		{
			"Blacklist 2 BLOCKED",
			`
			acl example.org {
				block type * net 192.168.0.0/16
			}`,
			[]string{},
			args{
				"www.example.org.",
				"192.168.0.2",
				dns.TypeAAAA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Blacklist 3 BLOCKED",
			`acl example.org {
				block type A
			}`,
			[]string{},
			args{
				"www.example.org.",
				"10.1.0.2",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Blacklist 3 ALLOWED",
			`acl example.org {
				block type A
			}`,
			[]string{},
			args{
				"www.example.org.",
				"10.1.0.2",
				dns.TypeAAAA,
			},
			dns.RcodeSuccess,
			false,
		},
		{
			"Blacklist 4 Single IP BLOCKED",
			`acl example.org {
				block type A net 192.168.1.2
			}`,
			[]string{},
			args{
				"www.example.org.",
				"192.168.1.2",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Blacklist 4 Single IP ALLOWED",
			`acl example.org {
				block type A net 192.168.1.2
			}`,
			[]string{},
			args{
				"www.example.org.",
				"192.168.1.3",
				dns.TypeA,
			},
			dns.RcodeSuccess,
			false,
		},
		{
			"Whitelist 1 ALLOWED",
			`acl example.org {
				allow net 192.168.0.0/16
				block
			}`,
			[]string{},
			args{
				"www.example.org.",
				"192.168.0.2",
				dns.TypeA,
			},
			dns.RcodeSuccess,
			false,
		},
		{
			"Whitelist 1 REFUSED",
			`acl example.org {
				allow type * net 192.168.0.0/16
				block
			}`,
			[]string{},
			args{
				"www.example.org.",
				"10.1.0.2",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Fine-Grained 1 REFUSED",
			`acl a.example.org {
				block type * net 192.168.1.0/24
			}`,
			[]string{"example.org"},
			args{
				"a.example.org.",
				"192.168.1.2",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Fine-Grained 1 ALLOWED",
			`acl a.example.org {
				block net 192.168.1.0/24
			}`,
			[]string{"example.org"},
			args{
				"www.example.org.",
				"192.168.1.2",
				dns.TypeA,
			},
			dns.RcodeSuccess,
			false,
		},
		{
			"Fine-Grained 2 REFUSED",
			`acl {
				block net 192.168.1.0/24
			}`,
			[]string{"example.org"},
			args{
				"a.example.org.",
				"192.168.1.2",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Fine-Grained 2 ALLOWED",
			`acl {
				block net 192.168.1.0/24
			}`,
			[]string{"example.org"},
			args{
				"a.example.com.",
				"192.168.1.2",
				dns.TypeA,
			},
			dns.RcodeSuccess,
			false,
		},
		{
			"Fine-Grained 3 REFUSED",
			`acl a.example.org {
				block net 192.168.1.0/24
			}
			acl b.example.org {
				block type * net 192.168.2.0/24
			}`,
			[]string{"example.org"},
			args{
				"b.example.org.",
				"192.168.2.2",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Fine-Grained 3 ALLOWED",
			`acl a.example.org {
				block net 192.168.1.0/24
			}
			acl b.example.org {
				block net 192.168.2.0/24
			}`,
			[]string{"example.org"},
			args{
				"b.example.org.",
				"192.168.1.2",
				dns.TypeA,
			},
			dns.RcodeSuccess,
			false,
		},
		// IPv6 tests.
		{
			"Blacklist 1 BLOCKED IPv6",
			`acl example.org {
				block type A net 2001:db8:abcd:0012::0/64
			}`,
			[]string{},
			args{
				"www.example.org.",
				"2001:db8:abcd:0012::1230",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Blacklist 1 ALLOWED IPv6",
			`acl example.org {
				block type A net 2001:db8:abcd:0012::0/64
			}`,
			[]string{},
			args{
				"www.example.org.",
				"2001:db8:abcd:0013::0",
				dns.TypeA,
			},
			dns.RcodeSuccess,
			false,
		},
		{
			"Blacklist 2 BLOCKED IPv6",
			`acl example.org {
				block type A
			}`,
			[]string{},
			args{
				"www.example.org.",
				"2001:0db8:85a3:0000:0000:8a2e:0370:7334",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Blacklist 3 Single IP BLOCKED IPv6",
			`acl example.org {
				block type A net 2001:0db8:85a3:0000:0000:8a2e:0370:7334
			}`,
			[]string{},
			args{
				"www.example.org.",
				"2001:0db8:85a3:0000:0000:8a2e:0370:7334",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Blacklist 3 Single IP ALLOWED IPv6",
			`acl example.org {
				block type A net 2001:0db8:85a3:0000:0000:8a2e:0370:7334
			}`,
			[]string{},
			args{
				"www.example.org.",
				"2001:0db8:85a3:0000:0000:8a2e:0370:7335",
				dns.TypeA,
			},
			dns.RcodeSuccess,
			false,
		},
		{
			"Fine-Grained 1 REFUSED IPv6",
			`acl a.example.org {
				block type * net 2001:db8:abcd:0012::0/64
			}`,
			[]string{"example.org"},
			args{
				"a.example.org.",
				"2001:db8:abcd:0012:2019::0",
				dns.TypeA,
			},
			dns.RcodeRefused,
			false,
		},
		{
			"Fine-Grained 1 ALLOWED IPv6",
			`acl a.example.org {
				block net 2001:db8:abcd:0012::0/64
			}`,
			[]string{"example.org"},
			args{
				"www.example.org.",
				"2001:db8:abcd:0012:2019::0",
				dns.TypeA,
			},
			dns.RcodeSuccess,
			false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctr := NewTestControllerWithZones(tt.config, tt.zones)
			a, err := parse(ctr)
			a.Next = test.NextHandler(dns.RcodeSuccess, nil)
			if err != nil {
				t.Errorf("Error: Cannot parse acl from config: %v", err)
				return
			}

			w := &testResponseWriter{}
			m := new(dns.Msg)
			w.setRemoteIP(tt.args.sourceIP)
			m.SetQuestion(tt.args.domain, tt.args.qtype)
			_, err = a.ServeDNS(ctx, w, m)
			if (err != nil) != tt.wantErr {
				t.Errorf("Error: acl.ServeDNS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if w.Rcode != tt.wantRcode {
				t.Errorf("Error: acl.ServeDNS() Rcode = %v, want %v", w.Rcode, tt.wantRcode)
			}
		})
	}
}
