package proxy

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/coredns/coredns/plugin/test"

	"github.com/mholt/caddy"
)

func TestAllowedDomain(t *testing.T) {
	upstream := &staticUpstream{
		from:              "miek.nl.",
		IgnoredSubDomains: []string{"download.miek.nl.", "static.miek.nl."}, // closing dot mandatory
	}
	tests := []struct {
		name     string
		expected bool
	}{
		{"miek.nl.", true},
		{"download.miek.nl.", false},
		{"static.miek.nl.", false},
		{"blaat.miek.nl.", true},
	}

	for i, test := range tests {
		isAllowed := upstream.IsAllowedDomain(test.name)
		if test.expected != isAllowed {
			t.Errorf("Test %d: expected %v found %v for %s", i+1, test.expected, isAllowed, test.name)
		}
	}
}

func TestProxyParse(t *testing.T) {
	rmFunc, cert, key, ca := getPEMFiles(t)
	defer rmFunc()

	grpc1 := "proxy . 8.8.8.8:53 {\n protocol grpc " + ca + "\n}"
	grpc2 := "proxy . 8.8.8.8:53 {\n protocol grpc " + cert + " " + key + "\n}"
	grpc3 := "proxy . 8.8.8.8:53 {\n protocol grpc " + cert + " " + key + " " + ca + "\n}"
	grpc4 := "proxy . 8.8.8.8:53 {\n protocol grpc " + key + "\n}"

	tests := []struct {
		inputUpstreams string
		shouldErr      bool
	}{
		{
			`proxy . 8.8.8.8:53`,
			false,
		},
		{
			`proxy 10.0.0.0/24 8.8.8.8:53`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    policy round_robin
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    fail_timeout 5s
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    max_fails 10
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    health_check /health:8080
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    except miek.nl example.org 10.0.0.0/24
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    spray
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    error_option
}`,
			true,
		},
		{
			`
proxy . some_bogus_filename`,
			true,
		},
		{
			`
proxy . 8.8.8.8:53 {
	protocol dns
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
	protocol grpc
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
	protocol grpc insecure
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
	protocol dns force_tcp
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
	protocol grpc a b c d
}`,
			true,
		},
		{
			grpc1,
			false,
		},
		{
			grpc2,
			false,
		},
		{
			grpc3,
			false,
		},
		{
			grpc4,
			true,
		},
		{
			`
proxy . 8.8.8.8:53 {
	protocol foobar
}`,
			true,
		},
		{
			`proxy`,
			true,
		},
		{
			`
proxy . 8.8.8.8:53 {
	protocol foobar
}`,
			true,
		},
		{
			`
proxy . 8.8.8.8:53 {
	policy
}`,
			true,
		},
		{
			`
proxy . 8.8.8.8:53 {
	fail_timeout
}`,
			true,
		},
		{
			`
proxy . 8.8.8.8:53 {
	fail_timeout junky
}`,
			true,
		},
		{
			`
proxy . 8.8.8.8:53 {
	health_check
}`,
			true,
		},
		{
			`
proxy . 8.8.8.8:53 {
	protocol dns force
}`,
			true,
		},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputUpstreams)
		_, err := NewStaticUpstreams(&c.Dispenser)
		if (err != nil) != test.shouldErr {
			t.Errorf("Test %d expected no error, got %v for %s", i+1, err, test.inputUpstreams)
		}
	}
}

func TestResolvParse(t *testing.T) {
	tests := []struct {
		inputUpstreams string
		filedata       string
		shouldErr      bool
		expected       []string
	}{
		{
			`
proxy . FILE
`,
			`
nameserver 1.2.3.4
nameserver 4.3.2.1
`,
			false,
			[]string{"1.2.3.4:53", "4.3.2.1:53"},
		},
		{
			`
proxy example.com 1.1.1.1:5000
proxy . FILE
proxy example.org 2.2.2.2:1234
`,
			`
nameserver 1.2.3.4
`,
			false,
			[]string{"1.1.1.1:5000", "1.2.3.4:53", "2.2.2.2:1234"},
		},
		{
			`
proxy example.com 1.1.1.1:5000
proxy . FILE
proxy example.org 2.2.2.2:1234
`,
			`
junky resolv.conf
`,
			false,
			[]string{"1.1.1.1:5000", "2.2.2.2:1234"},
		},
	}
	for i, tc := range tests {

		path, rm, err := test.TempFile(".", tc.filedata)
		if err != nil {
			t.Fatalf("Test %d could not create temp file %v", i, err)
		}
		defer rm()

		config := strings.Replace(tc.inputUpstreams, "FILE", path, -1)
		c := caddy.NewTestController("dns", config)
		upstreams, err := NewStaticUpstreams(&c.Dispenser)
		if (err != nil) != tc.shouldErr {
			t.Errorf("Test %d expected no error, got %v", i+1, err)
		}
		var hosts []string
		for _, u := range upstreams {
			for _, h := range u.(*staticUpstream).Hosts {
				hosts = append(hosts, h.Name)
			}
		}
		if !tc.shouldErr {
			if len(hosts) != len(tc.expected) {
				t.Errorf("Test %d expected %d hosts got %d", i+1, len(tc.expected), len(upstreams))
			} else {
				ok := true
				for i, v := range tc.expected {
					if v != hosts[i] {
						ok = false
					}
				}
				if !ok {
					t.Errorf("Test %d expected %v got %v", i+1, tc.expected, upstreams)
				}
			}
		}
	}
}

func TestMaxTo(t *testing.T) {
	// Has 16 IP addresses.
	config := `proxy . 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1 1.1.1.1`
	c := caddy.NewTestController("dns", config)
	_, err := NewStaticUpstreams(&c.Dispenser)
	if err == nil {
		t.Error("Expected to many TOs configured, but nil")
	}
}

func getPEMFiles(t *testing.T) (rmFunc func(), cert, key, ca string) {
	tempDir, rmFunc, err := test.WritePEMFiles("")
	if err != nil {
		t.Fatalf("Could not write PEM files: %s", err)
	}

	cert = filepath.Join(tempDir, "cert.pem")
	key = filepath.Join(tempDir, "key.pem")
	ca = filepath.Join(tempDir, "ca.pem")

	return
}
