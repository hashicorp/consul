package consul

import (
	"bytes"
	"crypto"
	crand "crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/memberlist"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/consul/proto/pbautoconf"
	"github.com/hashicorp/consul/proto/pbconfig"
	"github.com/hashicorp/consul/proto/pbconnect"
	"github.com/hashicorp/consul/proto/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/consul/types"

	"gopkg.in/square/go-jose.v2/jwt"
)

type mockAutoConfigBackend struct {
	mock.Mock
}

func (m *mockAutoConfigBackend) CreateACLToken(template *structs.ACLToken) (*structs.ACLToken, error) {
	ret := m.Called(template)
	// this handles converting an untyped nil to a typed nil
	token, _ := ret.Get(0).(*structs.ACLToken)
	return token, ret.Error(1)
}

func (m *mockAutoConfigBackend) DatacenterJoinAddresses(partition, segment string) ([]string, error) {
	ret := m.Called(partition, segment)
	// this handles converting an untyped nil to a typed nil
	addrs, _ := ret.Get(0).([]string)
	return addrs, ret.Error(1)
}

func (m *mockAutoConfigBackend) ForwardRPC(method string, req structs.RPCInfo, reply interface{}) (bool, error) {
	ret := m.Called(method, req, reply)
	return ret.Bool(0), ret.Error(1)
}

func (m *mockAutoConfigBackend) GetCARoots() (*structs.IndexedCARoots, error) {
	ret := m.Called()
	roots, _ := ret.Get(0).(*structs.IndexedCARoots)
	return roots, ret.Error(1)
}

func (m *mockAutoConfigBackend) SignCertificate(csr *x509.CertificateRequest, id connect.CertURI) (*structs.IssuedCert, error) {
	ret := m.Called(csr, id)
	cert, _ := ret.Get(0).(*structs.IssuedCert)
	return cert, ret.Error(1)
}

func testJWTStandardClaims() jwt.Claims {
	now := time.Now()

	return jwt.Claims{
		Subject:   "consul",
		Issuer:    "consul",
		Audience:  jwt.Audience{"consul"},
		NotBefore: jwt.NewNumericDate(now.Add(-1 * time.Second)),
		Expiry:    jwt.NewNumericDate(now.Add(10 * time.Minute)),
	}
}

func signJWT(t *testing.T, privKey string, claims jwt.Claims, privateClaims interface{}) string {
	t.Helper()
	token, err := oidcauthtest.SignJWT(privKey, claims, privateClaims)
	require.NoError(t, err)
	return token
}

func signJWTWithStandardClaims(t *testing.T, privKey string, claims interface{}) string {
	t.Helper()
	return signJWT(t, privKey, testJWTStandardClaims(), claims)
}

// TestAutoConfigInitialConfiguration is really an integration test of all the moving parts of the AutoConfig.InitialConfiguration RPC.
// Full testing of the individual parts will not be done in this test:
//
//   - Any implementations of the AutoConfigAuthorizer interface (although these test do use the jwtAuthorizer)
//   - Each of the individual config generation functions. These can be unit tested separately and should NOT
//     require running test servers
func TestAutoConfigInitialConfiguration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	gossipKey := make([]byte, 32)
	// this is not cryptographic randomness and is not secure but for the sake of this test its all we need.
	n, err := crand.Read(gossipKey)
	require.NoError(t, err)
	require.Equal(t, 32, n)

	gossipKeyEncoded := base64.StdEncoding.EncodeToString(gossipKey)

	// generate a test certificate for the server serving out the insecure RPC
	cert, key, cacert, err := testTLSCertificates("server.dc1.consul")
	require.NoError(t, err)

	// generate a JWT signer
	pub, priv, err := oidcauthtest.GenerateKey()
	require.NoError(t, err)

	_, altpriv, err := oidcauthtest.GenerateKey()
	require.NoError(t, err)

	// this CSR is what gets sent in the request
	csrID := connect.SpiffeIDAgent{
		Host:       "dummy.trustdomain",
		Agent:      "test-node",
		Datacenter: "dc1",
	}
	csr, _ := connect.TestCSR(t, &csrID)

	altCSRID := connect.SpiffeIDAgent{
		Host:       "dummy.trustdomain",
		Agent:      "alt",
		Datacenter: "dc1",
	}

	altCSR, _ := connect.TestCSR(t, &altCSRID)

	_, s, _ := testACLServerWithConfig(t, func(c *Config) {
		c.TLSConfig.Domain = "consul"
		c.AutoConfigAuthzEnabled = true
		c.AutoConfigAuthzAuthMethod = structs.ACLAuthMethod{
			Name:           "Auth Config Authorizer",
			Type:           "jwt",
			EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			Config: map[string]interface{}{
				"BoundAudiences":       []string{"consul"},
				"BoundIssuer":          "consul",
				"JWTValidationPubKeys": []string{pub},
				"ClaimMappings": map[string]string{
					"consul_node_name": "node",
				},
			},
		}
		c.AutoConfigAuthzClaimAssertions = []string{
			`value.node == "${node}"`,
		}
		c.AutoConfigAuthzAllowReuse = true

		cafile := path.Join(c.DataDir, "cacert.pem")
		err := os.WriteFile(cafile, []byte(cacert), 0600)
		require.NoError(t, err)

		certfile := path.Join(c.DataDir, "cert.pem")
		err = os.WriteFile(certfile, []byte(cert), 0600)
		require.NoError(t, err)

		keyfile := path.Join(c.DataDir, "key.pem")
		err = os.WriteFile(keyfile, []byte(key), 0600)
		require.NoError(t, err)

		c.TLSConfig.InternalRPC.CAFile = cafile
		c.TLSConfig.InternalRPC.CertFile = certfile
		c.TLSConfig.InternalRPC.KeyFile = keyfile
		c.TLSConfig.InternalRPC.VerifyOutgoing = true
		c.TLSConfig.InternalRPC.VerifyIncoming = true
		c.TLSConfig.InternalRPC.VerifyServerHostname = true
		c.TLSConfig.InternalRPC.TLSMinVersion = types.TLSv1_2

		c.ConnectEnabled = true
		c.AutoEncryptAllowTLS = true
		c.SerfLANConfig.MemberlistConfig.GossipVerifyIncoming = true
		c.SerfLANConfig.MemberlistConfig.GossipVerifyOutgoing = true

		keyring, err := memberlist.NewKeyring(nil, gossipKey)
		require.NoError(t, err)
		c.SerfLANConfig.MemberlistConfig.Keyring = keyring
	}, false)

	// TODO: use s.config.TLSConfig directly instead of creating a new one?
	conf := tlsutil.Config{
		InternalRPC: tlsutil.ProtocolConfig{
			CAFile:               s.config.TLSConfig.InternalRPC.CAFile,
			VerifyServerHostname: s.config.TLSConfig.InternalRPC.VerifyServerHostname,
			VerifyOutgoing:       s.config.TLSConfig.InternalRPC.VerifyOutgoing,
		},
		Domain: s.config.TLSConfig.Domain,
	}
	codec, err := insecureRPCClient(s, conf)
	require.NoError(t, err)

	waitForLeaderEstablishment(t, s)

	roots, err := s.getCARoots(nil, s.fsm.State())
	require.NoError(t, err)

	pbroots, err := pbconnect.NewCARootsFromStructs(roots)
	require.NoError(t, err)

	joinAddr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: s.config.SerfLANConfig.MemberlistConfig.AdvertisePort}

	// -------------------------------------------------------------------------
	// Common test setup is now complete
	// -------------------------------------------------------------------------

	type testCase struct {
		request       *pbautoconf.AutoConfigRequest
		expected      *pbautoconf.AutoConfigResponse
		patchResponse func(t *testing.T, srv *Server, resp *pbautoconf.AutoConfigResponse)
		err           string
	}

	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	cases := map[string]testCase{
		"wrong-datacenter": {
			request: &pbautoconf.AutoConfigRequest{
				Datacenter: "no-such-dc",
			},
			err: `invalid datacenter "no-such-dc" - agent auto configuration cannot target a remote datacenter`,
		},
		"unverifiable": {
			request: &pbautoconf.AutoConfigRequest{
				Node: "test-node",
				// this is signed using an incorrect private key
				JWT: signJWTWithStandardClaims(t, altpriv, map[string]interface{}{"consul_node_name": "test-node"}),
			},
			err: "Permission denied: Failed JWT authorization: no known key successfully validated the token signature",
		},
		"bad-req-node": {
			request: &pbautoconf.AutoConfigRequest{
				Node: "bad node",
				JWT:  signJWTWithStandardClaims(t, priv, map[string]interface{}{"consul_node_name": "test-node"}),
			},
			err: "Invalid request field. node =",
		},
		"bad-req-segment": {
			request: &pbautoconf.AutoConfigRequest{
				Node:    "test-node",
				Segment: "bad segment",
				JWT:     signJWTWithStandardClaims(t, priv, map[string]interface{}{"consul_node_name": "test-node"}),
			},
			err: "Invalid request field. segment =",
		},
		"bad-req-partition": {
			request: &pbautoconf.AutoConfigRequest{
				Node:      "test-node",
				Partition: "bad partition",
				JWT:       signJWTWithStandardClaims(t, priv, map[string]interface{}{"consul_node_name": "test-node"}),
			},
			err: "Invalid request field. partition =",
		},
		"claim-assertion-failed": {
			request: &pbautoconf.AutoConfigRequest{
				Node: "test-node",
				JWT:  signJWTWithStandardClaims(t, priv, map[string]interface{}{"wrong_claim": "test-node"}),
			},
			err: "Permission denied: Failed JWT claim assertion",
		},
		"bad-csr-id": {
			request: &pbautoconf.AutoConfigRequest{
				Node: "test-node",
				JWT:  signJWTWithStandardClaims(t, priv, map[string]interface{}{"consul_node_name": "test-node"}),
				CSR:  altCSR,
			},
			err: "Spiffe ID agent name (alt) of the certificate signing request is not for the correct node (test-node)",
		},
		"good": {
			request: &pbautoconf.AutoConfigRequest{
				Node: "test-node",
				JWT:  signJWTWithStandardClaims(t, priv, map[string]interface{}{"consul_node_name": "test-node"}),
				CSR:  csr,
			},
			expected: &pbautoconf.AutoConfigResponse{
				CARoots:             pbroots,
				ExtraCACertificates: []string{cacert},
				Config: &pbconfig.Config{
					Datacenter:        "dc1",
					PrimaryDatacenter: "dc1",
					NodeName:          "test-node",
					ACL: &pbconfig.ACL{
						Enabled:       true,
						PolicyTTL:     "30s",
						TokenTTL:      "30s",
						RoleTTL:       "30s",
						DownPolicy:    "extend-cache",
						DefaultPolicy: "deny",
						Tokens: &pbconfig.ACLTokens{
							Agent: "patched-secret",
						},
					},
					Gossip: &pbconfig.Gossip{
						Encryption: &pbconfig.GossipEncryption{
							Key:            gossipKeyEncoded,
							VerifyIncoming: true,
							VerifyOutgoing: true,
						},
						RetryJoinLAN: []string{joinAddr.String()},
					},
					TLS: &pbconfig.TLS{
						VerifyOutgoing:       true,
						VerifyServerHostname: true,
						MinVersion:           "tls12",
					},
				},
			},
			patchResponse: func(t *testing.T, _ *Server, resp *pbautoconf.AutoConfigResponse) {
				// we are expecting an ACL token but cannot check anything for equality
				// so here we check that it was set and overwrite it
				require.NotNil(t, resp.Config)
				require.NotNil(t, resp.Config.ACL)
				require.NotNil(t, resp.Config.ACL.Tokens)
				require.NotEmpty(t, resp.Config.ACL.Tokens.Agent)
				resp.Config.ACL.Tokens.Agent = "patched-secret"

				require.NotNil(t, resp.Certificate)
				require.NotEmpty(t, resp.Certificate.SerialNumber)
				require.NotEmpty(t, resp.Certificate.CertPEM)
				require.Empty(t, resp.Certificate.Service)
				require.Empty(t, resp.Certificate.ServiceURI)
				require.Equal(t, "test-node", resp.Certificate.Agent)

				expectedID := connect.SpiffeIDAgent{
					Host:       roots.TrustDomain,
					Agent:      "test-node",
					Partition:  defaultEntMeta.PartitionOrDefault(),
					Datacenter: "dc1",
				}

				require.Equal(t, expectedID.URI().String(), resp.Certificate.AgentURI)

				// nil this out so we don't check it for equality
				resp.Certificate = nil
			},
		},
	}

	for testName, tcase := range cases {
		t.Run(testName, func(t *testing.T) {
			reply := &pbautoconf.AutoConfigResponse{}
			err := msgpackrpc.CallWithCodec(codec, "AutoConfig.InitialConfiguration", &tcase.request, reply)
			if tcase.err != "" {
				testutil.RequireErrorContains(t, err, tcase.err)
			} else {
				require.NoError(t, err)
				if tcase.patchResponse != nil {
					tcase.patchResponse(t, s, reply)
				}
				prototest.AssertDeepEqual(t, tcase.expected, reply)
			}
		})
	}
}

func TestAutoConfig_baseConfig(t *testing.T) {
	type testCase struct {
		serverConfig Config
		opts         AutoConfigOptions
		expected     *pbautoconf.AutoConfigResponse
		err          string
	}

	cases := map[string]testCase{
		"ok": {
			serverConfig: Config{
				Datacenter:        "oSWzfhnU",
				PrimaryDatacenter: "53XO9mx4",
			},
			opts: AutoConfigOptions{
				NodeName:    "lBdc0lsH",
				SegmentName: "HZiwlWpi",
			},
			expected: &pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{
					Datacenter:        "oSWzfhnU",
					PrimaryDatacenter: "53XO9mx4",
					NodeName:          "lBdc0lsH",
					SegmentName:       "HZiwlWpi",
				},
			},
		},
		"no-node-name": {
			serverConfig: Config{
				Datacenter:        "oSWzfhnU",
				PrimaryDatacenter: "53XO9mx4",
			},
			err: "Cannot generate auto config response without a node name",
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ac := AutoConfig{
				config: &tcase.serverConfig,
			}

			actual := &pbautoconf.AutoConfigResponse{Config: &pbconfig.Config{}}
			err := ac.baseConfig(tcase.opts, actual)
			if tcase.err == "" {
				require.NoError(t, err)
				require.Equal(t, tcase.expected, actual)
			} else {
				testutil.RequireErrorContains(t, err, tcase.err)
			}
		})
	}
}

func TestAutoConfig_updateTLSSettingsInConfig(t *testing.T) {
	_, _, cacert, err := testTLSCertificates("server.dc1.consul")
	require.NoError(t, err)

	dir := testutil.TempDir(t, "auto-config-tls-settings")
	cafile := path.Join(dir, "cacert.pem")
	err = os.WriteFile(cafile, []byte(cacert), 0600)
	require.NoError(t, err)

	type testCase struct {
		tlsConfig tlsutil.Config
		expected  *pbautoconf.AutoConfigResponse
	}

	cases := map[string]testCase{
		"secure": {
			tlsConfig: tlsutil.Config{
				InternalRPC: tlsutil.ProtocolConfig{
					VerifyServerHostname: true,
					VerifyOutgoing:       true,
					TLSMinVersion:        types.TLSv1_2,
					CAFile:               cafile,
					CipherSuites:         []types.TLSCipherSuite{"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
				},
			},
			expected: &pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{
					TLS: &pbconfig.TLS{
						VerifyOutgoing:       true,
						VerifyServerHostname: true,
						MinVersion:           "tls12",
						CipherSuites:         "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
					},
				},
			},
		},
		"less-secure": {
			tlsConfig: tlsutil.Config{
				InternalRPC: tlsutil.ProtocolConfig{
					VerifyServerHostname: false,
					VerifyOutgoing:       true,
					TLSMinVersion:        types.TLSv1_0,
					CAFile:               cafile,
					CipherSuites:         []types.TLSCipherSuite{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
				},
			},
			expected: &pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{
					TLS: &pbconfig.TLS{
						VerifyOutgoing:       true,
						VerifyServerHostname: false,
						MinVersion:           "tls10",
						CipherSuites:         "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
					},
				},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			logger := testutil.Logger(t)
			configurator, err := tlsutil.NewConfigurator(tcase.tlsConfig, logger)
			require.NoError(t, err)

			ac := &AutoConfig{
				tlsConfigurator: configurator,
			}

			actual := &pbautoconf.AutoConfigResponse{Config: &pbconfig.Config{}}
			err = ac.updateTLSSettingsInConfig(AutoConfigOptions{}, actual)
			require.NoError(t, err)
			require.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAutoConfig_updateGossipEncryptionInConfig(t *testing.T) {
	type testCase struct {
		conf     memberlist.Config
		expected *pbautoconf.AutoConfigResponse
	}

	gossipKey := make([]byte, 32)
	// this is not cryptographic randomness and is not secure but for the sake of this test its all we need.
	n, err := crand.Read(gossipKey)
	require.NoError(t, err)
	require.Equal(t, 32, n)
	gossipKeyEncoded := base64.StdEncoding.EncodeToString(gossipKey)

	keyring, err := memberlist.NewKeyring(nil, gossipKey)
	require.NoError(t, err)

	cases := map[string]testCase{
		"encryption-required": {
			conf: memberlist.Config{
				Keyring:              keyring,
				GossipVerifyIncoming: true,
				GossipVerifyOutgoing: true,
			},
			expected: &pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{
					Gossip: &pbconfig.Gossip{
						Encryption: &pbconfig.GossipEncryption{
							Key:            gossipKeyEncoded,
							VerifyIncoming: true,
							VerifyOutgoing: true,
						},
					},
				},
			},
		},
		"encryption-allowed": {
			conf: memberlist.Config{
				Keyring:              keyring,
				GossipVerifyIncoming: false,
				GossipVerifyOutgoing: false,
			},
			expected: &pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{
					Gossip: &pbconfig.Gossip{
						Encryption: &pbconfig.GossipEncryption{
							Key:            gossipKeyEncoded,
							VerifyIncoming: false,
							VerifyOutgoing: false,
						},
					},
				},
			},
		},
		"encryption-disabled": {
			// zero values all around - if no keyring is configured then the gossip
			// encryption settings should not be set.
			expected: &pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.SerfLANConfig.MemberlistConfig = &tcase.conf

			ac := AutoConfig{
				config: cfg,
			}

			actual := &pbautoconf.AutoConfigResponse{Config: &pbconfig.Config{}}
			err := ac.updateGossipEncryptionInConfig(AutoConfigOptions{}, actual)
			require.NoError(t, err)
			require.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAutoConfig_updateTLSCertificatesInConfig(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	// Generate a Test CA
	ca := connect.TestCA(t, nil)

	// roots will be returned by the mock backend
	roots := structs.IndexedCARoots{
		ActiveRootID: ca.ID,
		TrustDomain:  connect.TestClusterID + ".consul",
		Roots: []*structs.CARoot{
			ca,
		},
	}

	// this CSR is what gets put into the opts for the
	// function to look at an process
	csrID := connect.SpiffeIDAgent{
		Host:       roots.TrustDomain,
		Agent:      "test",
		Datacenter: "dc1",
	}
	csrStr, _ := connect.TestCSR(t, &csrID)

	csr, err := connect.ParseCSR(csrStr)
	require.NoError(t, err)

	// fake certificate response for the backend
	fakeCert := structs.IssuedCert{
		SerialNumber:   "1",
		CertPEM:        "not-currently-decoded",
		ValidAfter:     now,
		ValidBefore:    later,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
		RaftIndex: structs.RaftIndex{
			ModifyIndex: 10,
			CreateIndex: 10,
		},
	}

	// translate the fake cert to the protobuf equivalent
	// for embedding in expected results
	pbcert, err := pbconnect.NewIssuedCertFromStructs(&fakeCert)
	require.NoError(t, err)

	// generate a CA certificate to use for specifying non-Connect
	// certificates which come back differently in the response
	_, _, cacert, err := testTLSCertificates("server.dc1.consul")
	require.NoError(t, err)

	// write out that ca cert to disk - it is unfortunate that
	// this is necessary but creation of the tlsutil.Configurator
	// will error if it cannot load the CA certificate from disk
	dir := testutil.TempDir(t, "auto-config-tls-certificate")
	cafile := path.Join(dir, "cacert.pem")
	err = os.WriteFile(cafile, []byte(cacert), 0600)
	require.NoError(t, err)

	// translate the roots response to protobuf to be embedded
	// into the expected results
	pbroots, err := pbconnect.NewCARootsFromStructs(&roots)
	require.NoError(t, err)

	type testCase struct {
		serverConfig Config
		tlsConfig    tlsutil.Config

		opts     AutoConfigOptions
		expected *pbautoconf.AutoConfigResponse
	}

	cases := map[string]testCase{
		"no-csr": {
			serverConfig: Config{
				ConnectEnabled: true,
			},
			tlsConfig: tlsutil.Config{
				InternalRPC: tlsutil.ProtocolConfig{
					VerifyServerHostname: true,
					VerifyOutgoing:       true,
					TLSMinVersion:        types.TLSv1_2,
					CAFile:               cafile,
					CipherSuites:         []types.TLSCipherSuite{"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
				},
			},
			expected: &pbautoconf.AutoConfigResponse{
				CARoots:             pbroots,
				ExtraCACertificates: []string{cacert},
				Config:              &pbconfig.Config{},
			},
		},
		"signed-certificate": {
			serverConfig: Config{
				ConnectEnabled: true,
			},
			tlsConfig: tlsutil.Config{
				InternalRPC: tlsutil.ProtocolConfig{
					VerifyServerHostname: true,
					VerifyOutgoing:       true,
					TLSMinVersion:        types.TLSv1_2,
					CAFile:               cafile,
					CipherSuites:         []types.TLSCipherSuite{"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
				},
			},
			opts: AutoConfigOptions{
				NodeName: "test",
				CSR:      csr,
				SpiffeID: &csrID,
			},
			expected: &pbautoconf.AutoConfigResponse{
				Config:              &pbconfig.Config{},
				CARoots:             pbroots,
				ExtraCACertificates: []string{cacert},
				Certificate:         pbcert,
			},
		},
		"connect-disabled": {
			serverConfig: Config{
				ConnectEnabled: false,
			},
			expected: &pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			backend := &mockAutoConfigBackend{}
			backend.On("GetCARoots").Return(&roots, nil)
			backend.On("SignCertificate", tcase.opts.CSR, tcase.opts.SpiffeID).Return(&fakeCert, nil)

			tlsConfigurator, err := tlsutil.NewConfigurator(tcase.tlsConfig, testutil.Logger(t))
			require.NoError(t, err)

			ac := AutoConfig{
				config:          &tcase.serverConfig,
				tlsConfigurator: tlsConfigurator,
				backend:         backend,
			}

			actual := &pbautoconf.AutoConfigResponse{Config: &pbconfig.Config{}}
			err = ac.updateTLSCertificatesInConfig(tcase.opts, actual)
			require.NoError(t, err)
			require.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAutoConfig_updateACLsInConfig(t *testing.T) {
	type testCase struct {
		config         Config
		expected       *pbautoconf.AutoConfigResponse
		expectACLToken bool
		err            error
	}

	const (
		tokenAccessor = "b98761aa-c0ee-445b-9b0c-f54b56b47778"
		tokenSecret   = "1c96448a-ab04-4caa-982a-e8b095a111e2"
	)

	testDC := "dc1"

	cases := map[string]testCase{
		"enabled": {
			config: Config{
				Datacenter:        testDC,
				PrimaryDatacenter: testDC,
				ACLsEnabled:       true,
				ACLResolverSettings: ACLResolverSettings{
					ACLPolicyTTL:     7 * time.Second,
					ACLRoleTTL:       10 * time.Second,
					ACLTokenTTL:      12 * time.Second,
					ACLDefaultPolicy: "allow",
					ACLDownPolicy:    "deny",
				},
				ACLEnableKeyListPolicy: true,
			},
			expectACLToken: true,
			expected: &pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{
					ACL: &pbconfig.ACL{
						Enabled:             true,
						PolicyTTL:           "7s",
						RoleTTL:             "10s",
						TokenTTL:            "12s",
						DownPolicy:          "deny",
						DefaultPolicy:       "allow",
						EnableKeyListPolicy: true,
						Tokens: &pbconfig.ACLTokens{
							Agent: tokenSecret,
						},
					},
				},
			},
		},
		"disabled": {
			config: Config{
				Datacenter:        testDC,
				PrimaryDatacenter: testDC,
				ACLsEnabled:       false,
				ACLResolverSettings: ACLResolverSettings{
					ACLPolicyTTL:     7 * time.Second,
					ACLRoleTTL:       10 * time.Second,
					ACLTokenTTL:      12 * time.Second,
					ACLDefaultPolicy: "allow",
					ACLDownPolicy:    "deny",
				},
				ACLEnableKeyListPolicy: true,
			},
			expectACLToken: false,
			expected: &pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{
					ACL: &pbconfig.ACL{
						Enabled:             false,
						PolicyTTL:           "7s",
						RoleTTL:             "10s",
						TokenTTL:            "12s",
						DownPolicy:          "deny",
						DefaultPolicy:       "allow",
						EnableKeyListPolicy: true,
					},
				},
			},
		},
		"local-tokens-disabled": {
			config: Config{
				Datacenter:        testDC,
				PrimaryDatacenter: "somewhere-else",
				ACLsEnabled:       true,
			},
			expectACLToken: true,
			err:            fmt.Errorf("Agent Auto Configuration requires local token usage to be enabled in this datacenter"),
		},
	}
	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			backend := &mockAutoConfigBackend{}
			expectedTemplate := &structs.ACLToken{
				Description: `Auto Config Token for Node "something"`,
				Local:       true,
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "something",
						Datacenter: testDC,
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}

			testToken := &structs.ACLToken{
				AccessorID:  tokenAccessor,
				SecretID:    tokenSecret,
				Description: `Auto Config Token for Node "something"`,
				Local:       true,
				NodeIdentities: []*structs.ACLNodeIdentity{
					{
						NodeName:   "something",
						Datacenter: testDC,
					},
				},
				EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
			}

			if tcase.expectACLToken {
				backend.On("CreateACLToken", expectedTemplate).Return(testToken, tcase.err).Once()
			}

			ac := AutoConfig{config: &tcase.config, backend: backend}

			actual := &pbautoconf.AutoConfigResponse{Config: &pbconfig.Config{}}
			err := ac.updateACLsInConfig(AutoConfigOptions{NodeName: "something"}, actual)
			if tcase.err != nil {
				testutil.RequireErrorContains(t, err, tcase.err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tcase.expected, actual)
			}

			backend.AssertExpectations(t)
		})
	}
}

func TestAutoConfig_updateJoinAddressesInConfig(t *testing.T) {
	addrs := []string{"198.18.0.7:8300", "198.18.0.1:8300"}
	backend := &mockAutoConfigBackend{}
	backend.On("DatacenterJoinAddresses", "", "").Return(addrs, nil).Once()

	ac := AutoConfig{backend: backend}

	actual := pbautoconf.AutoConfigResponse{Config: &pbconfig.Config{}}
	err := ac.updateJoinAddressesInConfig(AutoConfigOptions{}, &actual)
	require.NoError(t, err)

	require.NotNil(t, actual.Config.Gossip)
	require.ElementsMatch(t, addrs, actual.Config.Gossip.RetryJoinLAN)

	backend.AssertExpectations(t)
}

func TestAutoConfig_parseAutoConfigCSR(t *testing.T) {
	// createCSR copies the behavior of connect.CreateCSR with some
	// customizations to allow for better unit testing.
	createCSR := func(tmpl *x509.CertificateRequest, privateKey crypto.Signer) (string, error) {
		connect.HackSANExtensionForCSR(tmpl)
		bs, err := x509.CreateCertificateRequest(crand.Reader, tmpl, privateKey)
		require.NoError(t, err)
		var csrBuf bytes.Buffer
		err = pem.Encode(&csrBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: bs})
		require.NoError(t, err)
		return csrBuf.String(), nil
	}
	pk, _, err := connect.GeneratePrivateKey()
	require.NoError(t, err)

	agentURI := connect.SpiffeIDAgent{
		Host:       "test-host",
		Datacenter: "tdc1",
		Agent:      "test-agent",
	}.URI()

	tests := []struct {
		name      string
		setup     func() string
		expectErr string
	}{
		{
			name:      "err_garbage_data",
			expectErr: "Failed to parse CSR",
			setup:     func() string { return "garbage" },
		},
		{
			name:      "err_not_one_uri",
			expectErr: "CSR SAN contains an invalid number of URIs",
			setup: func() string {
				tmpl := &x509.CertificateRequest{
					URIs:               []*url.URL{agentURI, agentURI},
					SignatureAlgorithm: connect.SigAlgoForKey(pk),
				}
				csr, err := createCSR(tmpl, pk)
				require.NoError(t, err)
				return csr
			},
		},
		{
			name:      "err_email",
			expectErr: "CSR SAN does not allow specifying email addresses",
			setup: func() string {
				tmpl := &x509.CertificateRequest{
					URIs:               []*url.URL{agentURI},
					EmailAddresses:     []string{"test@example.com"},
					SignatureAlgorithm: connect.SigAlgoForKey(pk),
				}
				csr, err := createCSR(tmpl, pk)
				require.NoError(t, err)
				return csr
			},
		},
		{
			name:      "err_spiffe_parse_uri",
			expectErr: "Failed to parse the SPIFFE URI",
			setup: func() string {
				tmpl := &x509.CertificateRequest{
					URIs:               []*url.URL{connect.SpiffeIDAgent{}.URI()},
					SignatureAlgorithm: connect.SigAlgoForKey(pk),
				}
				csr, err := createCSR(tmpl, pk)
				require.NoError(t, err)
				return csr
			},
		},
		{
			name:      "err_not_agent",
			expectErr: "SPIFFE ID is not an Agent ID",
			setup: func() string {
				spiffe := connect.SpiffeIDService{
					Namespace:  "tns",
					Datacenter: "tdc1",
					Service:    "test-service",
				}
				tmpl := &x509.CertificateRequest{
					URIs:               []*url.URL{spiffe.URI()},
					SignatureAlgorithm: connect.SigAlgoForKey(pk),
				}
				csr, err := createCSR(tmpl, pk)
				require.NoError(t, err)
				return csr
			},
		},
		{
			name: "success",
			setup: func() string {
				tmpl := &x509.CertificateRequest{
					URIs:               []*url.URL{agentURI},
					SignatureAlgorithm: connect.SigAlgoForKey(pk),
				}
				csr, err := createCSR(tmpl, pk)
				require.NoError(t, err)
				return csr
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, spif, err := parseAutoConfigCSR(tc.setup())
			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
				// TODO better verification of these
				require.NotNil(t, req)
				require.NotNil(t, spif)
			}

		})
	}
}

func TestAutoConfig_invalidSegmentName(t *testing.T) {
	invalid := []string{
		"\n",
		"\r",
		"\t",
		"`",
		`'`,
		`"`,
		` `,
		`a b`,
		`a'b`,
		`a or b`,
		`a and b`,
		`segment name`,
		`segment"name`,
		`"segment"name`,
		`"segment" name`,
		`segment'name'`,
	}
	valid := []string{
		``,
		`a`,
		`a.b`,
		`a.b.c`,
		`a-b-c`,
		`segment.name`,
	}

	for _, s := range invalid {
		require.True(t, invalidSegmentName.MatchString(s), "incorrect match: %v", s)
	}
	for _, s := range valid {
		require.False(t, invalidSegmentName.MatchString(s), "incorrect match: %v", s)
	}
}
