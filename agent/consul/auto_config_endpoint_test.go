package consul

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/consul/proto/pbautoconf"
	"github.com/hashicorp/consul/proto/pbconfig"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/memberlist"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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

func (m *mockAutoConfigBackend) DatacenterJoinAddresses(segment string) ([]string, error) {
	ret := m.Called(segment)
	// this handles converting an untyped nil to a typed nil
	addrs, _ := ret.Get(0).([]string)
	return addrs, ret.Error(1)
}

func (m *mockAutoConfigBackend) ForwardRPC(method string, info structs.RPCInfo, args, reply interface{}) (bool, error) {
	ret := m.Called(method, info, args, reply)
	return ret.Bool(0), ret.Error(1)
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
//  * Any implementations of the AutoConfigAuthorizer interface (although these test do use the jwtAuthorizer)
//  * Each of the individual config generation functions. These can be unit tested separately and should NOT
//    require running test servers
func TestAutoConfigInitialConfiguration(t *testing.T) {
	type testCase struct {
		request       pbautoconf.AutoConfigRequest
		expected      pbautoconf.AutoConfigResponse
		patchResponse func(t *testing.T, srv *Server, resp *pbautoconf.AutoConfigResponse)
		err           string
	}

	gossipKey := make([]byte, 32)
	// this is not cryptographic randomness and is not secure but for the sake of this test its all we need.
	n, err := rand.Read(gossipKey)
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

	cases := map[string]testCase{
		"wrong-datacenter": {
			request: pbautoconf.AutoConfigRequest{
				Datacenter: "no-such-dc",
			},
			err: `invalid datacenter "no-such-dc" - agent auto configuration cannot target a remote datacenter`,
		},
		"unverifiable": {
			request: pbautoconf.AutoConfigRequest{
				Node: "test-node",
				// this is signed using an incorrect private key
				JWT: signJWTWithStandardClaims(t, altpriv, map[string]interface{}{"consul_node_name": "test-node"}),
			},
			err: "Permission denied: Failed JWT authorization: no known key successfully validated the token signature",
		},
		"claim-assertion-failed": {
			request: pbautoconf.AutoConfigRequest{
				Node: "test-node",
				JWT:  signJWTWithStandardClaims(t, priv, map[string]interface{}{"wrong_claim": "test-node"}),
			},
			err: "Permission denied: Failed JWT claim assertion",
		},
		"good": {
			request: pbautoconf.AutoConfigRequest{
				Node: "test-node",
				JWT:  signJWTWithStandardClaims(t, priv, map[string]interface{}{"consul_node_name": "test-node"}),
			},
			expected: pbautoconf.AutoConfigResponse{
				Config: &pbconfig.Config{
					Datacenter:        "dc1",
					PrimaryDatacenter: "dc1",
					NodeName:          "test-node",
					AutoEncrypt: &pbconfig.AutoEncrypt{
						TLS: true,
					},
					ACL: &pbconfig.ACL{
						Enabled:       true,
						PolicyTTL:     "30s",
						TokenTTL:      "30s",
						RoleTTL:       "30s",
						DisabledTTL:   "0s",
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
					},
					TLS: &pbconfig.TLS{
						VerifyOutgoing:           true,
						VerifyServerHostname:     true,
						MinVersion:               "tls12",
						PreferServerCipherSuites: true,
					},
				},
			},
			patchResponse: func(t *testing.T, srv *Server, resp *pbautoconf.AutoConfigResponse) {
				// we are expecting an ACL token but cannot check anything for equality
				// so here we check that it was set and overwrite it
				require.NotNil(t, resp.Config)
				require.NotNil(t, resp.Config.ACL)
				require.NotNil(t, resp.Config.ACL.Tokens)
				require.NotEmpty(t, resp.Config.ACL.Tokens.Agent)
				resp.Config.ACL.Tokens.Agent = "patched-secret"

				// we don't know the expected join address until we start up the test server
				joinAddr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: srv.config.SerfLANConfig.MemberlistConfig.AdvertisePort}
				require.NotNil(t, resp.Config.Gossip)
				require.Equal(t, []string{joinAddr.String()}, resp.Config.Gossip.RetryJoinLAN)
				resp.Config.Gossip.RetryJoinLAN = nil
			},
		},
	}

	_, s, _ := testACLServerWithConfig(t, func(c *Config) {
		c.Domain = "consul"
		c.AutoConfigAuthzEnabled = true
		c.AutoConfigAuthzAuthMethod = structs.ACLAuthMethod{
			Name:           "Auth Config Authorizer",
			Type:           "jwt",
			EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
		err := ioutil.WriteFile(cafile, []byte(cacert), 0600)
		require.NoError(t, err)

		certfile := path.Join(c.DataDir, "cert.pem")
		err = ioutil.WriteFile(certfile, []byte(cert), 0600)
		require.NoError(t, err)

		keyfile := path.Join(c.DataDir, "key.pem")
		err = ioutil.WriteFile(keyfile, []byte(key), 0600)
		require.NoError(t, err)

		c.CAFile = cafile
		c.CertFile = certfile
		c.KeyFile = keyfile
		c.VerifyOutgoing = true
		c.VerifyIncoming = true
		c.VerifyServerHostname = true
		c.TLSMinVersion = "tls12"
		c.TLSPreferServerCipherSuites = true

		c.ConnectEnabled = true
		c.AutoEncryptAllowTLS = true
		c.SerfLANConfig.MemberlistConfig.GossipVerifyIncoming = true
		c.SerfLANConfig.MemberlistConfig.GossipVerifyOutgoing = true

		keyring, err := memberlist.NewKeyring(nil, gossipKey)
		require.NoError(t, err)
		c.SerfLANConfig.MemberlistConfig.Keyring = keyring
	}, false)

	conf := tlsutil.Config{
		CAFile:               s.config.CAFile,
		VerifyServerHostname: s.config.VerifyServerHostname,
		VerifyOutgoing:       s.config.VerifyOutgoing,
		Domain:               s.config.Domain,
	}
	codec, err := insecureRPCClient(s, conf)
	require.NoError(t, err)

	waitForLeaderEstablishment(t, s)

	for testName, tcase := range cases {
		t.Run(testName, func(t *testing.T) {
			var reply pbautoconf.AutoConfigResponse
			err := msgpackrpc.CallWithCodec(codec, "AutoConfig.InitialConfiguration", &tcase.request, &reply)
			if tcase.err != "" {
				testutil.RequireErrorContains(t, err, tcase.err)
			} else {
				require.NoError(t, err)
				if tcase.patchResponse != nil {
					tcase.patchResponse(t, s, &reply)
				}
				require.Equal(t, tcase.expected, reply)
			}
		})
	}
}

func TestAutoConfig_baseConfig(t *testing.T) {
	type testCase struct {
		serverConfig Config
		opts         AutoConfigOptions
		expected     pbconfig.Config
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
			expected: pbconfig.Config{
				Datacenter:        "oSWzfhnU",
				PrimaryDatacenter: "53XO9mx4",
				NodeName:          "lBdc0lsH",
				SegmentName:       "HZiwlWpi",
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

			var actual pbconfig.Config
			err := ac.baseConfig(tcase.opts, &actual)
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
	t.Cleanup(func() { os.RemoveAll(dir) })

	cafile := path.Join(dir, "cacert.pem")
	err = ioutil.WriteFile(cafile, []byte(cacert), 0600)
	require.NoError(t, err)

	parseCiphers := func(t *testing.T, cipherStr string) []uint16 {
		t.Helper()
		ciphers, err := tlsutil.ParseCiphers(cipherStr)
		require.NoError(t, err)
		return ciphers
	}

	type testCase struct {
		tlsConfig tlsutil.Config
		expected  pbconfig.Config
	}

	cases := map[string]testCase{
		"secure": {
			tlsConfig: tlsutil.Config{
				VerifyOutgoing:           true,
				VerifyServerHostname:     true,
				TLSMinVersion:            "tls12",
				PreferServerCipherSuites: true,
				CAFile:                   cafile,
				CipherSuites:             parseCiphers(t, "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"),
			},
			expected: pbconfig.Config{
				TLS: &pbconfig.TLS{
					VerifyOutgoing:           true,
					VerifyServerHostname:     true,
					MinVersion:               "tls12",
					PreferServerCipherSuites: true,
					CipherSuites:             "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				},
			},
		},
		"less-secure": {
			tlsConfig: tlsutil.Config{
				VerifyOutgoing:           true,
				VerifyServerHostname:     false,
				TLSMinVersion:            "tls10",
				PreferServerCipherSuites: false,
				CAFile:                   cafile,
				CipherSuites:             parseCiphers(t, "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"),
			},
			expected: pbconfig.Config{
				TLS: &pbconfig.TLS{
					VerifyOutgoing:           true,
					VerifyServerHostname:     false,
					MinVersion:               "tls10",
					PreferServerCipherSuites: false,
					CipherSuites:             "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
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

			var actual pbconfig.Config
			err = ac.updateTLSSettingsInConfig(AutoConfigOptions{}, &actual)
			require.NoError(t, err)
			require.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAutoConfig_updateGossipEncryptionInConfig(t *testing.T) {
	type testCase struct {
		conf     memberlist.Config
		expected pbconfig.Config
	}

	gossipKey := make([]byte, 32)
	// this is not cryptographic randomness and is not secure but for the sake of this test its all we need.
	n, err := rand.Read(gossipKey)
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
			expected: pbconfig.Config{
				Gossip: &pbconfig.Gossip{
					Encryption: &pbconfig.GossipEncryption{
						Key:            gossipKeyEncoded,
						VerifyIncoming: true,
						VerifyOutgoing: true,
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
			expected: pbconfig.Config{
				Gossip: &pbconfig.Gossip{
					Encryption: &pbconfig.GossipEncryption{
						Key:            gossipKeyEncoded,
						VerifyIncoming: false,
						VerifyOutgoing: false,
					},
				},
			},
		},
		"encryption-disabled": {
			// zero values all around - if no keyring is configured then the gossip
			// encryption settings should not be set.
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.SerfLANConfig.MemberlistConfig = &tcase.conf

			ac := AutoConfig{
				config: cfg,
			}

			var actual pbconfig.Config
			err := ac.updateGossipEncryptionInConfig(AutoConfigOptions{}, &actual)
			require.NoError(t, err)
			require.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAutoConfig_updateTLSCertificatesInConfig(t *testing.T) {
	type testCase struct {
		serverConfig Config
		expected     pbconfig.Config
	}

	cases := map[string]testCase{
		"auto_encrypt-enabled": {
			serverConfig: Config{
				ConnectEnabled:      true,
				AutoEncryptAllowTLS: true,
			},
			expected: pbconfig.Config{
				AutoEncrypt: &pbconfig.AutoEncrypt{TLS: true},
			},
		},
		"auto_encrypt-disabled": {
			serverConfig: Config{
				ConnectEnabled:      true,
				AutoEncryptAllowTLS: false,
			},
			expected: pbconfig.Config{
				AutoEncrypt: &pbconfig.AutoEncrypt{TLS: false},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ac := AutoConfig{
				config: &tcase.serverConfig,
			}

			var actual pbconfig.Config
			err := ac.updateTLSCertificatesInConfig(AutoConfigOptions{}, &actual)
			require.NoError(t, err)
			require.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAutoConfig_updateACLsInConfig(t *testing.T) {
	type testCase struct {
		config         Config
		expected       pbconfig.Config
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
				Datacenter:             testDC,
				PrimaryDatacenter:      testDC,
				ACLsEnabled:            true,
				ACLPolicyTTL:           7 * time.Second,
				ACLRoleTTL:             10 * time.Second,
				ACLTokenTTL:            12 * time.Second,
				ACLDisabledTTL:         31 * time.Second,
				ACLDefaultPolicy:       "allow",
				ACLDownPolicy:          "deny",
				ACLEnableKeyListPolicy: true,
			},
			expectACLToken: true,
			expected: pbconfig.Config{
				ACL: &pbconfig.ACL{
					Enabled:             true,
					PolicyTTL:           "7s",
					RoleTTL:             "10s",
					TokenTTL:            "12s",
					DisabledTTL:         "31s",
					DownPolicy:          "deny",
					DefaultPolicy:       "allow",
					EnableKeyListPolicy: true,
					Tokens: &pbconfig.ACLTokens{
						Agent: tokenSecret,
					},
				},
			},
		},
		"disabled": {
			config: Config{
				Datacenter:             testDC,
				PrimaryDatacenter:      testDC,
				ACLsEnabled:            false,
				ACLPolicyTTL:           7 * time.Second,
				ACLRoleTTL:             10 * time.Second,
				ACLTokenTTL:            12 * time.Second,
				ACLDisabledTTL:         31 * time.Second,
				ACLDefaultPolicy:       "allow",
				ACLDownPolicy:          "deny",
				ACLEnableKeyListPolicy: true,
			},
			expectACLToken: false,
			expected: pbconfig.Config{
				ACL: &pbconfig.ACL{
					Enabled:             false,
					PolicyTTL:           "7s",
					RoleTTL:             "10s",
					TokenTTL:            "12s",
					DisabledTTL:         "31s",
					DownPolicy:          "deny",
					DefaultPolicy:       "allow",
					EnableKeyListPolicy: true,
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
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
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
				EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
			}

			if tcase.expectACLToken {
				backend.On("CreateACLToken", expectedTemplate).Return(testToken, tcase.err).Once()
			}

			ac := AutoConfig{config: &tcase.config, backend: backend}

			var actual pbconfig.Config
			err := ac.updateACLsInConfig(AutoConfigOptions{NodeName: "something"}, &actual)
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
	backend.On("DatacenterJoinAddresses", "").Return(addrs, nil).Once()

	ac := AutoConfig{backend: backend}

	var actual pbconfig.Config
	err := ac.updateJoinAddressesInConfig(AutoConfigOptions{}, &actual)
	require.NoError(t, err)

	require.NotNil(t, actual.Gossip)
	require.ElementsMatch(t, addrs, actual.Gossip.RetryJoinLAN)

	backend.AssertExpectations(t)
}
