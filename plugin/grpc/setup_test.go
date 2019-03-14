package grpc

import (
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mholt/caddy"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		input           string
		shouldErr       bool
		expectedFrom    string
		expectedIgnored []string
		expectedErr     string
	}{
		// positive
		{"grpc . 127.0.0.1", false, ".", nil, ""},
		{"grpc . 127.0.0.1 {\nexcept miek.nl\n}\n", false, ".", nil, ""},
		{"grpc . 127.0.0.1", false, ".", nil, ""},
		{"grpc . 127.0.0.1:53", false, ".", nil, ""},
		{"grpc . 127.0.0.1:8080", false, ".", nil, ""},
		{"grpc . [::1]:53", false, ".", nil, ""},
		{"grpc . [2003::1]:53", false, ".", nil, ""},
		// negative
		{"grpc . a27.0.0.1", true, "", nil, "not an IP"},
		{"grpc . 127.0.0.1 {\nblaatl\n}\n", true, "", nil, "unknown property"},
		{`grpc . ::1
		grpc com ::2`, true, "", nil, "plugin"},
	}

	for i, test := range tests {
		c := caddy.NewTestController("grpc", test.input)
		g, err := parseGRPC(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: expected no error but found one for input %s, got: %v", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("Test %d: expected error to contain: %v, found error: %v, input: %s", i, test.expectedErr, err, test.input)
			}
		}

		if !test.shouldErr && g.from != test.expectedFrom {
			t.Errorf("Test %d: expected: %s, got: %s", i, test.expectedFrom, g.from)
		}
		if !test.shouldErr && test.expectedIgnored != nil {
			if !reflect.DeepEqual(g.ignored, test.expectedIgnored) {
				t.Errorf("Test %d: expected: %q, actual: %q", i, test.expectedIgnored, g.ignored)
			}
		}
	}
}

func TestSetupTLS(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedServerName string
		expectedErr        string
	}{
		// positive
		{`grpc . 127.0.0.1 {
tls_servername dns
}`, false, "dns", ""},
		{`grpc . 127.0.0.1 {
tls_servername dns
}`, false, "", ""},
		{`grpc . 127.0.0.1 {
tls
}`, false, "", ""},
		{`grpc . 127.0.0.1`, false, "", ""},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		g, err := parseGRPC(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: expected no error but found one for input %s, got: %v", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("Test %d: expected error to contain: %v, found error: %v, input: %s", i, test.expectedErr, err, test.input)
			}
		}

		if !test.shouldErr && test.expectedServerName != "" && g.tlsConfig != nil && test.expectedServerName != g.tlsConfig.ServerName {
			t.Errorf("Test %d: expected: %q, actual: %q", i, test.expectedServerName, g.tlsConfig.ServerName)
		}
	}
}

func TestSetupResolvconf(t *testing.T) {
	const resolv = "resolv.conf"
	if err := ioutil.WriteFile(resolv,
		[]byte(`nameserver 10.10.255.252
nameserver 10.10.255.253`), 0666); err != nil {
		t.Fatalf("Failed to write resolv.conf file: %s", err)
	}
	defer os.Remove(resolv)

	tests := []struct {
		input         string
		shouldErr     bool
		expectedErr   string
		expectedNames []string
	}{
		// pass
		{`grpc . ` + resolv, false, "", []string{"10.10.255.252:53", "10.10.255.253:53"}},
	}

	for i, test := range tests {
		c := caddy.NewTestController("grpc", test.input)
		f, err := parseGRPC(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: expected error but found %s for input %s", i, err, test.input)
			continue
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: expected no error but found one for input %s, got: %v", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("Test %d: expected error to contain: %v, found error: %v, input: %s", i, test.expectedErr, err, test.input)
			}
		}

		if !test.shouldErr {
			for j, n := range test.expectedNames {
				addr := f.proxies[j].addr
				if n != addr {
					t.Errorf("Test %d, expected %q, got %q", j, n, addr)
				}
			}
		}
	}
}
