package tls

import (
	"crypto/tls"
	"strings"
	"testing"

	"github.com/coredns/coredns/core/dnsserver"

	"github.com/mholt/caddy"
)

func TestTLS(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedRoot       string // expected root, set to the controller. Empty for negative cases.
		expectedErrContent string // substring from the expected error. Empty for positive cases.
	}{
		// positive
		// negative
		{"tls test_cert.pem test_key.pem test_ca.pem {\nunknown\n}", true, "", "unknown option"},
		// client_auth takes exactly one parameter, which must be one of known keywords.
		{"tls test_cert.pem test_key.pem test_ca.pem {\nclient_auth\n}", true, "", "Wrong argument"},
		{"tls test_cert.pem test_key.pem test_ca.pem {\nclient_auth none bogus\n}", true, "", "Wrong argument"},
		{"tls test_cert.pem test_key.pem test_ca.pem {\nclient_auth bogus\n}", true, "", "unknown authentication type"},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		err := setup(c)
		//cfg := dnsserver.GetConfig(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}
		}
	}
}

func TestTLSClientAuthentication(t *testing.T) {
	// Invalid configurations are tested in the general test case.  In this test we only look into specific details of valid client_auth options.
	tests := []struct {
		option string			// tls plugin option(s)
		expectedType tls.ClientAuthType // expected authentication type.
	}{
		// By default, or if 'nocert' is specified, no cert should be requested.
		// Other cases should be a straightforward mapping from the keyword to the type value.
		{"", tls.NoClientCert},
		{"{\nclient_auth nocert\n}", tls.NoClientCert},
		{"{\nclient_auth request\n}", tls.RequestClientCert},
		{"{\nclient_auth require\n}", tls.RequireAnyClientCert},
		{"{\nclient_auth verify_if_given\n}", tls.VerifyClientCertIfGiven},
		{"{\nclient_auth require_and_verify\n}", tls.RequireAndVerifyClientCert},
	}

	for i, test := range tests {
		input := "tls test_cert.pem test_key.pem test_ca.pem " + test.option
		c := caddy.NewTestController("dns", input)
		err := setup(c)
		if err != nil {
			t.Errorf("Test %d: TLS config is unexpectedly rejected: %v", i, err)
			continue // there's no point in the rest of the tests.
		}
		cfg := dnsserver.GetConfig(c)
		if cfg.TLSConfig.ClientCAs == nil {
			t.Errorf("Test %d: Client CA is not configured", i)
		}
		if cfg.TLSConfig.ClientAuth != test.expectedType {
			t.Errorf("Test %d: Unexpected client auth type: %d", i, cfg.TLSConfig.ClientAuth)
		}
	}
}
