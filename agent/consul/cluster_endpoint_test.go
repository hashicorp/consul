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

	"github.com/hashicorp/consul/agent/agentpb"
	"github.com/hashicorp/consul/agent/agentpb/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/go-sso/oidcauth/oidcauthtest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/memberlist"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/stretchr/testify/require"

	"gopkg.in/square/go-jose.v2/jwt"
)

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

// TestClusterAutoConfig is really an integration test of all the moving parts of the Cluster.AutoConfig RPC.
// Full testing of the individual parts will not be done in this test:
//
//  * Any implementations of the AutoConfigAuthorizer interface (although these test do use the jwtAuthorizer)
//  * Each of the individual config generation functions. These can be unit tested separately and many wont
//    require a running test server.
func TestClusterAutoConfig(t *testing.T) {
	type testCase struct {
		request       agentpb.AutoConfigRequest
		expected      agentpb.AutoConfigResponse
		patchResponse func(t *testing.T, srv *Server, resp *agentpb.AutoConfigResponse)
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
			request: agentpb.AutoConfigRequest{
				Datacenter: "no-such-dc",
			},
			err: `invalid datacenter "no-such-dc" - agent auto configuration cannot target a remote datacenter`,
		},
		"unverifiable": {
			request: agentpb.AutoConfigRequest{
				Node: "test-node",
				// this is signed using an incorrect private key
				JWT: signJWTWithStandardClaims(t, altpriv, map[string]interface{}{"consul_node_name": "test-node"}),
			},
			err: "Permission denied: Failed JWT authorization: no known key successfully validated the token signature",
		},
		"claim-assertion-failed": {
			request: agentpb.AutoConfigRequest{
				Node: "test-node",
				JWT:  signJWTWithStandardClaims(t, priv, map[string]interface{}{"wrong_claim": "test-node"}),
			},
			err: "Permission denied: Failed JWT claim assertion",
		},
		"good": {
			request: agentpb.AutoConfigRequest{
				Node: "test-node",
				JWT:  signJWTWithStandardClaims(t, priv, map[string]interface{}{"consul_node_name": "test-node"}),
			},
			expected: agentpb.AutoConfigResponse{
				Config: &config.Config{
					Datacenter:        "dc1",
					PrimaryDatacenter: "dc1",
					NodeName:          "test-node",
					AutoEncrypt: &config.AutoEncrypt{
						TLS: true,
					},
					ACL: &config.ACL{
						Enabled:       true,
						PolicyTTL:     "30s",
						TokenTTL:      "30s",
						RoleTTL:       "30s",
						DisabledTTL:   "0s",
						DownPolicy:    "extend-cache",
						DefaultPolicy: "deny",
						Tokens: &config.ACLTokens{
							Agent: "patched-secret",
						},
					},
					EncryptKey:                  gossipKeyEncoded,
					EncryptVerifyIncoming:       true,
					EncryptVerifyOutgoing:       true,
					VerifyOutgoing:              true,
					VerifyServerHostname:        true,
					TLSMinVersion:               "tls12",
					TLSPreferServerCipherSuites: true,
				},
			},
			patchResponse: func(t *testing.T, srv *Server, resp *agentpb.AutoConfigResponse) {
				// we are expecting an ACL token but cannot check anything for equality
				// so here we check that it was set and overwrite it
				require.NotNil(t, resp.Config)
				require.NotNil(t, resp.Config.ACL)
				require.NotNil(t, resp.Config.ACL.Tokens)
				require.NotEmpty(t, resp.Config.ACL.Tokens.Agent)
				resp.Config.ACL.Tokens.Agent = "patched-secret"

				// we don't know the expected join address until we start up the test server
				joinAddr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: srv.config.SerfLANConfig.MemberlistConfig.AdvertisePort}
				require.Equal(t, []string{joinAddr.String()}, resp.Config.RetryJoinLAN)
				resp.Config.RetryJoinLAN = nil
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
			var reply agentpb.AutoConfigResponse
			err := msgpackrpc.CallWithCodec(codec, "Cluster.AutoConfig", &tcase.request, &reply)
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

func TestClusterAutoConfig_baseConfig(t *testing.T) {
	type testCase struct {
		serverConfig Config
		opts         AutoConfigOptions
		expected     config.Config
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
			expected: config.Config{
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
			cluster := Cluster{
				srv: &Server{
					config: &tcase.serverConfig,
				},
			}

			var actual config.Config
			err := cluster.baseConfig(tcase.opts, &actual)
			if tcase.err == "" {
				require.NoError(t, err)
				require.Equal(t, tcase.expected, actual)
			} else {
				testutil.RequireErrorContains(t, err, tcase.err)
			}
		})
	}
}

func TestClusterAutoConfig_updateTLSSettingsInConfig(t *testing.T) {
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
		expected  config.Config
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
			expected: config.Config{
				VerifyOutgoing:              true,
				VerifyServerHostname:        true,
				TLSMinVersion:               "tls12",
				TLSPreferServerCipherSuites: true,
				TLSCipherSuites:             "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
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
			expected: config.Config{
				VerifyOutgoing:              true,
				VerifyServerHostname:        false,
				TLSMinVersion:               "tls10",
				TLSPreferServerCipherSuites: false,
				TLSCipherSuites:             "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			logger := testutil.Logger(t)
			configurator, err := tlsutil.NewConfigurator(tcase.tlsConfig, logger)
			require.NoError(t, err)

			cluster := &Cluster{
				srv: &Server{
					tlsConfigurator: configurator,
				},
			}

			var actual config.Config
			err = cluster.updateTLSSettingsInConfig(AutoConfigOptions{}, &actual)
			require.NoError(t, err)
			require.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAutoConfig_updateGossipEncryptionInConfig(t *testing.T) {
	type testCase struct {
		conf     memberlist.Config
		expected config.Config
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
			expected: config.Config{
				EncryptKey:            gossipKeyEncoded,
				EncryptVerifyIncoming: true,
				EncryptVerifyOutgoing: true,
			},
		},
		"encryption-allowed": {
			conf: memberlist.Config{
				Keyring:              keyring,
				GossipVerifyIncoming: false,
				GossipVerifyOutgoing: false,
			},
			expected: config.Config{
				EncryptKey:            gossipKeyEncoded,
				EncryptVerifyIncoming: false,
				EncryptVerifyOutgoing: false,
			},
		},
		"encryption-disabled": {
			// zero values all around - if no keyring is configured then the gossip
			// encryption settings should not be set.
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			cluster := Cluster{
				srv: &Server{
					config: DefaultConfig(),
				},
			}

			cluster.srv.config.SerfLANConfig.MemberlistConfig = &tcase.conf

			var actual config.Config
			err := cluster.updateGossipEncryptionInConfig(AutoConfigOptions{}, &actual)
			require.NoError(t, err)
			require.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAutoConfig_updateTLSCertificatesInConfig(t *testing.T) {
	type testCase struct {
		serverConfig Config
		expected     config.Config
	}

	cases := map[string]testCase{
		"auto_encrypt-enabled": {
			serverConfig: Config{
				ConnectEnabled:      true,
				AutoEncryptAllowTLS: true,
			},
			expected: config.Config{
				AutoEncrypt: &config.AutoEncrypt{TLS: true},
			},
		},
		"auto_encrypt-disabled": {
			serverConfig: Config{
				ConnectEnabled:      true,
				AutoEncryptAllowTLS: false,
			},
			expected: config.Config{
				AutoEncrypt: &config.AutoEncrypt{TLS: false},
			},
		},
		"connect-disabled": {
			serverConfig: Config{
				ConnectEnabled:      false,
				AutoEncryptAllowTLS: true,
			},
			expected: config.Config{
				AutoEncrypt: &config.AutoEncrypt{TLS: false},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			cluster := Cluster{
				srv: &Server{
					config: &tcase.serverConfig,
				},
			}

			var actual config.Config
			err := cluster.updateTLSCertificatesInConfig(AutoConfigOptions{}, &actual)
			require.NoError(t, err)
			require.Equal(t, tcase.expected, actual)
		})
	}
}

func TestAutoConfig_updateACLsInConfig(t *testing.T) {
	type testCase struct {
		patch    func(c *Config)
		expected config.Config
		verify   func(t *testing.T, c *config.Config)
		err      string
	}

	cases := map[string]testCase{
		"enabled": {
			patch: func(c *Config) {
				c.ACLsEnabled = true
				c.ACLPolicyTTL = 7 * time.Second
				c.ACLRoleTTL = 10 * time.Second
				c.ACLTokenTTL = 12 * time.Second
				c.ACLDisabledTTL = 31 * time.Second
				c.ACLDefaultPolicy = "allow"
				c.ACLDownPolicy = "deny"
				c.ACLEnableKeyListPolicy = true
			},
			expected: config.Config{
				ACL: &config.ACL{
					Enabled:             true,
					PolicyTTL:           "7s",
					RoleTTL:             "10s",
					TokenTTL:            "12s",
					DisabledTTL:         "31s",
					DownPolicy:          "deny",
					DefaultPolicy:       "allow",
					EnableKeyListPolicy: true,
					Tokens:              &config.ACLTokens{Agent: "verified"},
				},
			},
			verify: func(t *testing.T, c *config.Config) {
				t.Helper()
				// the agent token secret is non-deterministically generated
				// So we want to validate that one was set and overwrite with
				// a value that the expected configurate wants.
				require.NotNil(t, c)
				require.NotNil(t, c.ACL)
				require.NotNil(t, c.ACL.Tokens)
				require.NotEmpty(t, c.ACL.Tokens.Agent)
				c.ACL.Tokens.Agent = "verified"
			},
		},
		"disabled": {
			patch: func(c *Config) {
				c.ACLsEnabled = false
				c.ACLPolicyTTL = 7 * time.Second
				c.ACLRoleTTL = 10 * time.Second
				c.ACLTokenTTL = 12 * time.Second
				c.ACLDisabledTTL = 31 * time.Second
				c.ACLDefaultPolicy = "allow"
				c.ACLDownPolicy = "deny"
				c.ACLEnableKeyListPolicy = true
			},
			expected: config.Config{
				ACL: &config.ACL{
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
			patch: func(c *Config) {
				c.PrimaryDatacenter = "somewhere else"
			},
			err: "Agent Auto Configuration requires local token usage to be enabled in this datacenter",
		},
	}
	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			_, s, _ := testACLServerWithConfig(t, tcase.patch, false)

			waitForLeaderEstablishment(t, s)

			cluster := Cluster{srv: s}

			var actual config.Config
			err := cluster.updateACLsInConfig(AutoConfigOptions{NodeName: "something"}, &actual)
			if tcase.err != "" {
				testutil.RequireErrorContains(t, err, tcase.err)
			} else {
				require.NoError(t, err)
				if tcase.verify != nil {
					tcase.verify(t, &actual)
				}
				require.Equal(t, tcase.expected, actual)
			}
		})
	}
}

func TestAutoConfig_updateJoinAddressesInConfig(t *testing.T) {
	conf := testClusterConfig{
		Datacenter: "primary",
		Servers:    3,
	}

	nodes := newTestCluster(t, &conf)

	cluster := Cluster{srv: nodes.Servers[0]}

	var actual config.Config
	err := cluster.updateJoinAddressesInConfig(AutoConfigOptions{}, &actual)
	require.NoError(t, err)

	var expected []string
	for _, srv := range nodes.Servers {
		expected = append(expected, fmt.Sprintf("127.0.0.1:%d", srv.config.SerfLANConfig.MemberlistConfig.BindPort))
	}
	require.ElementsMatch(t, expected, actual.RetryJoinLAN)
}
