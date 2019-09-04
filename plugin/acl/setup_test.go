package acl

import (
	"testing"

	"github.com/caddyserver/caddy"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		// IPv4 tests.
		{
			"Blacklist 1",
			`acl {
				block type A net 192.168.0.0/16
			}`,
			false,
		},
		{
			"Blacklist 2",
			`acl {
				block type * net 192.168.0.0/16
			}`,
			false,
		},
		{
			"Blacklist 3",
			`acl {
				block type A net *
			}`,
			false,
		},
		{
			"Blacklist 4",
			`acl {
				allow type * net 192.168.1.0/24
				block type * net 192.168.0.0/16
			}`,
			false,
		},
		{
			"Whitelist 1",
			`acl {
				allow type * net 192.168.0.0/16
				block type * net *
			}`,
			false,
		},
		{
			"fine-grained 1",
			`acl a.example.org {
				block type * net 192.168.1.0/24
			}`,
			false,
		},
		{
			"fine-grained 2",
			`acl a.example.org {
				block type * net 192.168.1.0/24
			}
			acl b.example.org {
				block type * net 192.168.2.0/24
			}`,
			false,
		},
		{
			"Multiple Networks 1",
			`acl example.org {
				block type * net 192.168.1.0/24 192.168.3.0/24
			}`,
			false,
		},
		{
			"Multiple Qtypes 1",
			`acl example.org {
				block type TXT ANY CNAME net 192.168.3.0/24
			}`,
			false,
		},
		{
			"Missing argument 1",
			`acl {
				block A net 192.168.0.0/16
			}`,
			true,
		},
		{
			"Missing argument 2",
			`acl {
				block type net 192.168.0.0/16
			}`,
			true,
		},
		{
			"Illegal argument 1",
			`acl {
				block type ABC net 192.168.0.0/16
			}`,
			true,
		},
		{
			"Illegal argument 2",
			`acl {
				blck type A net 192.168.0.0/16
			}`,
			true,
		},
		{
			"Illegal argument 3",
			`acl {
				block type A net 192.168.0/16
			}`,
			true,
		},
		{
			"Illegal argument 4",
			`acl {
				block type A net 192.168.0.0/33
			}`,
			true,
		},
		// IPv6 tests.
		{
			"Blacklist 1 IPv6",
			`acl {
				block type A net 2001:0db8:85a3:0000:0000:8a2e:0370:7334
			}`,
			false,
		},
		{
			"Blacklist 2 IPv6",
			`acl {
				block type * net 2001:db8:85a3::8a2e:370:7334
			}`,
			false,
		},
		{
			"Blacklist 3 IPv6",
			`acl {
				block type A
			}`,
			false,
		},
		{
			"Blacklist 4 IPv6",
			`acl {
				allow net 2001:db8:abcd:0012::0/64
				block net 2001:db8:abcd:0012::0/48
			}`,
			false,
		},
		{
			"Whitelist 1 IPv6",
			`acl {
				allow net 2001:db8:abcd:0012::0/64
				block
			}`,
			false,
		},
		{
			"fine-grained 1 IPv6",
			`acl a.example.org {
				block net 2001:db8:abcd:0012::0/64
			}`,
			false,
		},
		{
			"fine-grained 2 IPv6",
			`acl a.example.org {
				block net 2001:db8:abcd:0012::0/64
			}
			acl b.example.org {
				block net 2001:db8:abcd:0013::0/64
			}`,
			false,
		},
		{
			"Multiple Networks 1 IPv6",
			`acl example.org {
				block net 2001:db8:abcd:0012::0/64 2001:db8:85a3::8a2e:370:7334/64
			}`,
			false,
		},
		{
			"Illegal argument 1 IPv6",
			`acl {
				block type A net 2001::85a3::8a2e:370:7334
			}`,
			true,
		},
		{
			"Illegal argument 2 IPv6",
			`acl {
				block type A net 2001:db8:85a3:::8a2e:370:7334
			}`,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctr := caddy.NewTestController("dns", tt.config)
			if err := setup(ctr); (err != nil) != tt.wantErr {
				t.Errorf("Error: setup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	type args struct {
		rawNet string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Network range 1",
			args{"10.218.10.8/24"},
			"10.218.10.8/24",
		},
		{
			"IP address 1",
			args{"10.218.10.8"},
			"10.218.10.8/32",
		},
		{
			"IPv6 address 1",
			args{"2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
			"2001:0db8:85a3:0000:0000:8a2e:0370:7334/128",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalize(tt.args.rawNet); got != tt.want {
				t.Errorf("Error: normalize() = %v, want %v", got, tt.want)
			}
		})
	}
}
