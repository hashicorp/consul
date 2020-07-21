package consul

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestAutoEncrypt_resolveAddr(t *testing.T) {
	type args struct {
		rawHost string
		logger  hclog.Logger
	}
	logger := testutil.Logger(t)

	tests := []struct {
		name    string
		args    args
		ips     []net.IP
		wantErr bool
	}{
		{
			name: "host without port",
			args: args{
				"127.0.0.1",
				logger,
			},
			ips:     []net.IP{net.IPv4(127, 0, 0, 1)},
			wantErr: false,
		},
		{
			name: "host with port",
			args: args{
				"127.0.0.1:1234",
				logger,
			},
			ips:     []net.IP{net.IPv4(127, 0, 0, 1)},
			wantErr: false,
		},
		{
			name: "host with broken port",
			args: args{
				"127.0.0.1:xyz",
				logger,
			},
			ips:     []net.IP{net.IPv4(127, 0, 0, 1)},
			wantErr: false,
		},
		{
			name: "not an address",
			args: args{
				"abc",
				logger,
			},
			ips:     nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := resolveAddr(tt.args.rawHost, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveAddr error: %v, wantErr: %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.ips, ips)
		})
	}
}

func TestAutoEncrypt_missingPortError(t *testing.T) {
	host := "127.0.0.1"
	_, _, err := net.SplitHostPort(host)
	require.True(t, missingPortError(host, err))

	host = "127.0.0.1:1234"
	_, _, err = net.SplitHostPort(host)
	require.False(t, missingPortError(host, err))
}

func TestAutoEncrypt_RequestAutoEncryptCerts(t *testing.T) {
	dir1, c1 := testClient(t)
	defer os.RemoveAll(dir1)
	defer c1.Shutdown()
	servers := []string{"localhost"}
	port := 8301
	token := ""

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(75*time.Millisecond))
	defer cancel()

	doneCh := make(chan struct{})
	var err error
	go func() {
		_, err = c1.RequestAutoEncryptCerts(ctx, servers, port, token, nil, nil)
		close(doneCh)
	}()
	select {
	case <-doneCh:
		// since there are no servers at this port, we shouldn't be
		// done and this should be an error of some sorts that happened
		// in the setup phase before entering the for loop in
		// RequestAutoEncryptCerts.
		require.NoError(t, err)
	case <-ctx.Done():
		// this is the happy case since auto encrypt is in its loop to
		// try to request certs.
	}
}

func TestAutoEncrypt_autoEncryptCSR(t *testing.T) {
	type testCase struct {
		conf         *Config
		extraDNSSANs []string
		extraIPSANs  []net.IP
		err          string

		// to validate the csr
		expectedSubject  pkix.Name
		expectedSigAlg   x509.SignatureAlgorithm
		expectedPubAlg   x509.PublicKeyAlgorithm
		expectedDNSNames []string
		expectedIPs      []net.IP
		expectedURIs     []*url.URL
	}

	cases := map[string]testCase{
		"sans": {
			conf: &Config{
				Datacenter: "dc1",
				NodeName:   "test-node",
				CAConfig:   &structs.CAConfiguration{},
			},
			extraDNSSANs: []string{"foo.local", "bar.local"},
			extraIPSANs:  []net.IP{net.IPv4(198, 18, 0, 1), net.IPv4(198, 18, 0, 2)},
			expectedSubject: pkix.Name{
				CommonName: connect.AgentCN("test-node", dummyTrustDomain),
				Names: []pkix.AttributeTypeAndValue{
					{
						// 2,5,4,3 is the CommonName type ASN1 identifier
						Type:  asn1.ObjectIdentifier{2, 5, 4, 3},
						Value: "testnode.agnt.dummy.tr.consul",
					},
				},
			},
			expectedSigAlg: x509.ECDSAWithSHA256,
			expectedPubAlg: x509.ECDSA,
			expectedDNSNames: []string{
				"localhost",
				"foo.local",
				"bar.local",
			},
			expectedIPs: []net.IP{
				{127, 0, 0, 1},
				net.ParseIP("::1"),
				{198, 18, 0, 1},
				{198, 18, 0, 2},
			},
			expectedURIs: []*url.URL{
				{
					Scheme: "spiffe",
					Host:   dummyTrustDomain,
					Path:   "/agent/client/dc/dc1/id/test-node",
				},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			client := Client{config: tcase.conf}

			_, csr, err := client.autoEncryptCSR(tcase.extraDNSSANs, tcase.extraIPSANs)
			if tcase.err == "" {
				require.NoError(t, err)

				request, err := connect.ParseCSR(csr)
				require.NoError(t, err)
				require.NotNil(t, request)

				require.Equal(t, tcase.expectedSubject, request.Subject)
				require.Equal(t, tcase.expectedSigAlg, request.SignatureAlgorithm)
				require.Equal(t, tcase.expectedPubAlg, request.PublicKeyAlgorithm)
				require.Equal(t, tcase.expectedDNSNames, request.DNSNames)
				require.Equal(t, tcase.expectedIPs, request.IPAddresses)
				require.Equal(t, tcase.expectedURIs, request.URIs)
			} else {
				require.Error(t, err)
				require.Empty(t, csr)
			}
		})
	}
}
