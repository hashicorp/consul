package autoconf

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestAutoEncrypt_generateCSR(t *testing.T) {
	type testCase struct {
		conf *config.RuntimeConfig

		// to validate the csr
		expectedSubject  pkix.Name
		expectedSigAlg   x509.SignatureAlgorithm
		expectedPubAlg   x509.PublicKeyAlgorithm
		expectedDNSNames []string
		expectedIPs      []net.IP
		expectedURIs     []*url.URL
	}

	cases := map[string]testCase{
		"ip-sans": {
			conf: &config.RuntimeConfig{
				Datacenter:       "dc1",
				NodeName:         "test-node",
				AutoEncryptTLS:   true,
				AutoEncryptIPSAN: []net.IP{net.IPv4(198, 18, 0, 1), net.IPv4(198, 18, 0, 2)},
			},
			expectedSubject:  pkix.Name{},
			expectedSigAlg:   x509.ECDSAWithSHA256,
			expectedPubAlg:   x509.ECDSA,
			expectedDNSNames: defaultDNSSANs,
			expectedIPs: append(defaultIPSANs,
				net.IP{198, 18, 0, 1},
				net.IP{198, 18, 0, 2},
			),
			expectedURIs: []*url.URL{
				{
					Scheme: "spiffe",
					Host:   unknownTrustDomain,
					Path:   "/agent/client/dc/dc1/id/test-node",
				},
			},
		},
		"dns-sans": {
			conf: &config.RuntimeConfig{
				Datacenter:        "dc1",
				NodeName:          "test-node",
				AutoEncryptTLS:    true,
				AutoEncryptDNSSAN: []string{"foo.local", "bar.local"},
			},
			expectedSubject:  pkix.Name{},
			expectedSigAlg:   x509.ECDSAWithSHA256,
			expectedPubAlg:   x509.ECDSA,
			expectedDNSNames: append(defaultDNSSANs, "foo.local", "bar.local"),
			expectedIPs:      defaultIPSANs,
			expectedURIs: []*url.URL{
				{
					Scheme: "spiffe",
					Host:   unknownTrustDomain,
					Path:   "/agent/client/dc/dc1/id/test-node",
				},
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ac := AutoConfig{config: tcase.conf}

			csr, _, err := ac.generateCSR()
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
		})
	}
}

func TestAutoEncrypt_hosts(t *testing.T) {
	type testCase struct {
		serverProvider ServerProvider
		config         *config.RuntimeConfig

		hosts []string
		err   string
	}

	providerNone := newMockServerProvider(t)
	providerNone.On("FindLANServer").Return(nil).Times(0)

	providerWithServer := newMockServerProvider(t)
	providerWithServer.On("FindLANServer").Return(&metadata.Server{Addr: &net.TCPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 1234}}).Times(0)

	cases := map[string]testCase{
		"router-override": {
			serverProvider: providerWithServer,
			config: &config.RuntimeConfig{
				RetryJoinLAN: []string{"127.0.0.1:9876", "192.168.1.2:4321"},
			},
			hosts: []string{"198.18.0.1:1234"},
		},
		"various-addresses": {
			serverProvider: providerNone,
			config: &config.RuntimeConfig{
				RetryJoinLAN: []string{
					"192.168.1.1:5432",
					"start.local",
					"[::ffff:172.16.5.4]",
					"main.dev:6789",
					"198.18.0.1",
					"foo.com",
					"[2001:db8::1234]:1234",
					"abc.local:9876",
				},
			},
			hosts: []string{
				"192.168.1.1",
				"start.local",
				"[::ffff:172.16.5.4]",
				"main.dev",
				"198.18.0.1",
				"foo.com",
				"2001:db8::1234",
				"abc.local",
			},
		},
		"split-host-port-error": {
			serverProvider: providerNone,
			config: &config.RuntimeConfig{
				RetryJoinLAN: []string{"this-is-not:a:ip:and_port"},
			},
			err: "no auto-encrypt server addresses available for use",
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ac := AutoConfig{
				config: tcase.config,
				logger: testutil.Logger(t),
				acConfig: Config{
					ServerProvider: tcase.serverProvider,
				},
			}

			hosts, err := ac.joinHosts()
			if tcase.err != "" {
				testutil.RequireErrorContains(t, err, tcase.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tcase.hosts, hosts)
			}
		})
	}
}

func TestAutoEncrypt_InitialCerts(t *testing.T) {
	token := "1a148388-3dd7-4db4-9eea-520424b4a86a"
	datacenter := "foo"
	nodeName := "bar"

	mcfg := newMockedConfig(t)

	_, indexedRoots, cert := testCerts(t, nodeName, datacenter)

	// The following are called once for each round through the auto-encrypt initial certs outer loop
	// (not the per-host direct rpc attempts but the one involving the RetryWaiter)
	mcfg.tokens.On("AgentToken").Return(token).Times(2)
	mcfg.serverProvider.On("FindLANServer").Return(nil).Times(2)

	request := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: token},
		Datacenter:   datacenter,
		// this gets removed by the mock code as its non-deterministic what it will be
		CSR: "",
	}

	// first failure
	mcfg.directRPC.On("RPC",
		datacenter,
		nodeName,
		&net.TCPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 8300},
		"AutoEncrypt.Sign",
		&request,
		&structs.SignedResponse{},
	).Once().Return(fmt.Errorf("injected error"))
	// second failure
	mcfg.directRPC.On("RPC",
		datacenter,
		nodeName,
		&net.TCPAddr{IP: net.IPv4(198, 18, 0, 2), Port: 8300},
		"AutoEncrypt.Sign",
		&request,
		&structs.SignedResponse{},
	).Once().Return(fmt.Errorf("injected error"))
	// third times is successfuly (second attempt to first server)
	mcfg.directRPC.On("RPC",
		datacenter,
		nodeName,
		&net.TCPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 8300},
		"AutoEncrypt.Sign",
		&request,
		&structs.SignedResponse{},
	).Once().Return(nil).Run(func(args mock.Arguments) {
		resp, ok := args.Get(5).(*structs.SignedResponse)
		require.True(t, ok)
		resp.ConnectCARoots = *indexedRoots
		resp.IssuedCert = *cert
		resp.VerifyServerHostname = true
	})

	mcfg.Config.Waiter = &retry.Waiter{MinFailures: 2, MaxWait: time.Millisecond}

	ac := AutoConfig{
		config: &config.RuntimeConfig{
			Datacenter:   datacenter,
			NodeName:     nodeName,
			RetryJoinLAN: []string{"198.18.0.1:1234", "198.18.0.2:3456"},
			ServerPort:   8300,
		},
		acConfig: mcfg.Config,
		logger:   testutil.Logger(t),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := ac.autoEncryptInitialCerts(ctx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, resp.VerifyServerHostname)
	require.NotEmpty(t, resp.IssuedCert.PrivateKeyPEM)
	resp.IssuedCert.PrivateKeyPEM = ""
	cert.PrivateKeyPEM = ""
	require.Equal(t, cert, &resp.IssuedCert)
	require.Equal(t, indexedRoots, &resp.ConnectCARoots)
	require.Empty(t, resp.ManualCARoots)
}

func TestAutoEncrypt_InitialConfiguration(t *testing.T) {
	token := "010494ae-ee45-4433-903c-a58c91297714"
	nodeName := "auto-encrypt"
	datacenter := "dc1"

	mcfg := newMockedConfig(t)
	loader := setupRuntimeConfig(t)
	loader.addConfigHCL(`
		auto_encrypt {
			tls = true
		}
	`)
	loader.opts.FlagValues.NodeName = &nodeName
	mcfg.Config.Loader = loader.Load

	indexedRoots, cert, extraCerts := mcfg.setupInitialTLS(t, nodeName, datacenter, token)

	// prepopulation is going to grab the token to populate the correct cache key
	mcfg.tokens.On("AgentToken").Return(token).Times(0)

	// no server provider
	mcfg.serverProvider.On("FindLANServer").Return(&metadata.Server{Addr: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8300}}).Times(1)

	populateResponse := func(args mock.Arguments) {
		resp, ok := args.Get(5).(*structs.SignedResponse)
		require.True(t, ok)
		*resp = structs.SignedResponse{
			VerifyServerHostname: true,
			ConnectCARoots:       *indexedRoots,
			IssuedCert:           *cert,
			ManualCARoots:        extraCerts,
		}
	}

	expectedRequest := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: token},
		Datacenter:   datacenter,
		// TODO (autoconf) Maybe in the future we should populate a CSR
		// and do some manual parsing/verification of the contents. The
		// bits not having to do with the signing key such as the requested
		// SANs and CN. For now though the mockDirectRPC type will empty
		// the CSR so we have to pass in an empty string to the expectation.
		CSR: "",
	}

	mcfg.directRPC.On(
		"RPC",
		datacenter,
		nodeName,
		&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8300},
		"AutoEncrypt.Sign",
		&expectedRequest,
		&structs.SignedResponse{}).Return(nil).Run(populateResponse)

	ac, err := New(mcfg.Config)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)

}

func TestAutoEncrypt_TokenUpdate(t *testing.T) {
	testAC := startedAutoConfig(t, true)

	newToken := "1a4cc445-86ed-46b4-a355-bbf5a11dddb0"

	rootsCtx, rootsCancel := context.WithCancel(context.Background())
	testAC.mcfg.cache.On("Notify",
		mock.Anything,
		cachetype.ConnectCARootName,
		&structs.DCSpecificRequest{Datacenter: testAC.ac.config.Datacenter},
		rootsWatchID,
		mock.Anything,
	).Return(nil).Once().Run(func(args mock.Arguments) {
		rootsCancel()
	})

	leafCtx, leafCancel := context.WithCancel(context.Background())
	testAC.mcfg.cache.On("Notify",
		mock.Anything,
		cachetype.ConnectCALeafName,
		&cachetype.ConnectCALeafRequest{
			Datacenter: "dc1",
			Agent:      "autoconf",
			Token:      newToken,
			DNSSAN:     defaultDNSSANs,
			IPSAN:      defaultIPSANs,
		},
		leafWatchID,
		mock.Anything,
	).Return(nil).Once().Run(func(args mock.Arguments) {
		leafCancel()
	})

	// this will be retrieved once when resetting the leaf cert watch
	testAC.mcfg.tokens.On("AgentToken").Return(newToken).Once()

	// send the notification about the token update
	testAC.tokenUpdates <- struct{}{}

	// wait for the leaf cert watches
	require.True(t, waitForChans(100*time.Millisecond, leafCtx.Done(), rootsCtx.Done()), "New cache watches were not started within 100ms")
}

func TestAutoEncrypt_RootsUpdate(t *testing.T) {
	testAC := startedAutoConfig(t, true)

	secondCA := connect.TestCA(t, testAC.initialRoots.Roots[0])
	secondRoots := structs.IndexedCARoots{
		ActiveRootID: secondCA.ID,
		TrustDomain:  connect.TestClusterID,
		Roots: []*structs.CARoot{
			secondCA,
			testAC.initialRoots.Roots[0],
		},
		QueryMeta: structs.QueryMeta{
			Index: 99,
		},
	}

	updatedCtx, cancel := context.WithCancel(context.Background())
	testAC.mcfg.tlsCfg.On("UpdateAutoTLSCA",
		[]string{secondCA.RootCert, testAC.initialRoots.Roots[0].RootCert},
	).Return(nil).Once().Run(func(args mock.Arguments) {
		cancel()
	})

	// when a cache event comes in we end up recalculating the fallback timer which requires this call
	testAC.mcfg.tlsCfg.On("AutoEncryptCert").Return(&x509.Certificate{
		NotAfter: time.Now().Add(10 * time.Minute),
	}).Once()

	req := structs.DCSpecificRequest{Datacenter: "dc1"}
	require.True(t, testAC.mcfg.cache.sendNotification(context.Background(), req.CacheInfo().Key, cache.UpdateEvent{
		CorrelationID: rootsWatchID,
		Result:        &secondRoots,
		Meta: cache.ResultMeta{
			Index: secondRoots.Index,
		},
	}))

	require.True(t, waitForChans(100*time.Millisecond, updatedCtx.Done()), "TLS certificates were not updated within the alotted time")
}

func TestAutoEncrypt_CertUpdate(t *testing.T) {
	testAC := startedAutoConfig(t, true)
	secondCert := newLeaf(t, "autoconf", "dc1", testAC.initialRoots.Roots[0], 99, 10*time.Minute)

	updatedCtx, cancel := context.WithCancel(context.Background())
	testAC.mcfg.tlsCfg.On("UpdateAutoTLSCert",
		secondCert.CertPEM,
		"redacted",
	).Return(nil).Once().Run(func(args mock.Arguments) {
		cancel()
	})

	// when a cache event comes in we end up recalculating the fallback timer which requires this call
	testAC.mcfg.tlsCfg.On("AutoEncryptCert").Return(&x509.Certificate{
		NotAfter: secondCert.ValidBefore,
	}).Once()

	req := cachetype.ConnectCALeafRequest{
		Datacenter: "dc1",
		Agent:      "autoconf",
		Token:      testAC.originalToken,
		DNSSAN:     defaultDNSSANs,
		IPSAN:      defaultIPSANs,
	}
	require.True(t, testAC.mcfg.cache.sendNotification(context.Background(), req.CacheInfo().Key, cache.UpdateEvent{
		CorrelationID: leafWatchID,
		Result:        secondCert,
		Meta: cache.ResultMeta{
			Index: secondCert.ModifyIndex,
		},
	}))

	require.True(t, waitForChans(100*time.Millisecond, updatedCtx.Done()), "TLS certificates were not updated within the alotted time")
}

func TestAutoEncrypt_Fallback(t *testing.T) {
	testAC := startedAutoConfig(t, true)

	// at this point everything is operating normally and we are just
	// waiting for events. We are going to send a new cert that is basically
	// already expired and then allow the fallback routine to kick in.
	secondCert := newLeaf(t, "autoconf", "dc1", testAC.initialRoots.Roots[0], 100, time.Nanosecond)
	secondCA := connect.TestCA(t, testAC.initialRoots.Roots[0])
	secondRoots := structs.IndexedCARoots{
		ActiveRootID: secondCA.ID,
		TrustDomain:  connect.TestClusterID,
		Roots: []*structs.CARoot{
			secondCA,
			testAC.initialRoots.Roots[0],
		},
		QueryMeta: structs.QueryMeta{
			Index: 101,
		},
	}
	thirdCert := newLeaf(t, "autoconf", "dc1", secondCA, 102, 10*time.Minute)

	// setup the expectation for when the certs get updated initially
	updatedCtx, updateCancel := context.WithCancel(context.Background())
	testAC.mcfg.tlsCfg.On("UpdateAutoTLSCert",
		secondCert.CertPEM,
		"redacted",
	).Return(nil).Once().Run(func(args mock.Arguments) {
		updateCancel()
	})

	// when a cache event comes in we end up recalculating the fallback timer which requires this call
	testAC.mcfg.tlsCfg.On("AutoEncryptCert").Return(&x509.Certificate{
		NotAfter: secondCert.ValidBefore,
	}).Times(2)

	fallbackCtx, fallbackCancel := context.WithCancel(context.Background())

	// also testing here that we can change server IPs for ongoing operations
	testAC.mcfg.serverProvider.On("FindLANServer").Once().Return(&metadata.Server{
		Addr: &net.TCPAddr{IP: net.IPv4(198, 18, 23, 2), Port: 8300},
	})

	// after sending the notification for the cert update another InitialConfiguration RPC
	// will be made to pull down the latest configuration. So we need to set up the response
	// for the second RPC
	populateResponse := func(args mock.Arguments) {
		resp, ok := args.Get(5).(*structs.SignedResponse)
		require.True(t, ok)
		*resp = structs.SignedResponse{
			VerifyServerHostname: true,
			ConnectCARoots:       secondRoots,
			IssuedCert:           *thirdCert,
			ManualCARoots:        testAC.extraCerts,
		}

		fallbackCancel()
	}

	expectedRequest := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: testAC.originalToken},
		Datacenter:   "dc1",
		// TODO (autoconf) Maybe in the future we should populate a CSR
		// and do some manual parsing/verification of the contents. The
		// bits not having to do with the signing key such as the requested
		// SANs and CN. For now though the mockDirectRPC type will empty
		// the CSR so we have to pass in an empty string to the expectation.
		CSR: "",
	}

	// the fallback routine to perform auto-encrypt again will need to grab this
	testAC.mcfg.tokens.On("AgentToken").Return(testAC.originalToken).Once()

	testAC.mcfg.directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(198, 18, 23, 2), Port: 8300},
		"AutoEncrypt.Sign",
		&expectedRequest,
		&structs.SignedResponse{}).Return(nil).Run(populateResponse).Once()

	testAC.mcfg.expectInitialTLS(t, "autoconf", "dc1", testAC.originalToken, secondCA, &secondRoots, thirdCert, testAC.extraCerts)

	// after the second RPC we now will use the new certs validity period in the next run loop iteration
	testAC.mcfg.tlsCfg.On("AutoEncryptCert").Return(&x509.Certificate{
		NotAfter: time.Now().Add(10 * time.Minute),
	}).Once()

	// now that all the mocks are set up we can trigger the whole thing by sending the second expired cert
	// as a cache update event.
	req := cachetype.ConnectCALeafRequest{
		Datacenter: "dc1",
		Agent:      "autoconf",
		Token:      testAC.originalToken,
		DNSSAN:     defaultDNSSANs,
		IPSAN:      defaultIPSANs,
	}
	require.True(t, testAC.mcfg.cache.sendNotification(context.Background(), req.CacheInfo().Key, cache.UpdateEvent{
		CorrelationID: leafWatchID,
		Result:        secondCert,
		Meta: cache.ResultMeta{
			Index: secondCert.ModifyIndex,
		},
	}))

	// wait for the TLS certificates to get updated
	require.True(t, waitForChans(100*time.Millisecond, updatedCtx.Done()), "TLS certificates were not updated within the alotted time")

	// now wait for the fallback routine to be invoked
	require.True(t, waitForChans(100*time.Millisecond, fallbackCtx.Done()), "fallback routines did not get invoked within the alotted time")
}
