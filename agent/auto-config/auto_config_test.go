// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package autoconf

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto/private/pbautoconf"
	"github.com/hashicorp/consul/proto/private/pbconfig"
	"github.com/hashicorp/consul/sdk/testutil"
	testretry "github.com/hashicorp/consul/sdk/testutil/retry"
)

type configLoader struct {
	opts config.LoadOpts
}

func (c *configLoader) Load(source config.Source) (config.LoadResult, error) {
	opts := c.opts
	opts.DefaultConfig = source
	return config.Load(opts)
}

func (c *configLoader) addConfigHCL(cfg string) {
	c.opts.HCL = append(c.opts.HCL, cfg)
}

func requireChanNotReady(t *testing.T, ch <-chan struct{}) {
	select {
	case <-ch:
		require.Fail(t, "chan is ready when it shouldn't be")
	default:
		return
	}
}

func requireChanReady(t *testing.T, ch <-chan struct{}) {
	select {
	case <-ch:
		return
	default:
		require.Fail(t, "chan is not ready when it should be")
	}
}

func waitForChan(timer *time.Timer, ch <-chan struct{}) bool {
	select {
	case <-timer.C:
		return false
	case <-ch:
		return true
	}
}

func waitForChans(timeout time.Duration, chans ...<-chan struct{}) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for _, ch := range chans {
		if !waitForChan(timer, ch) {
			return false
		}
	}
	return true
}

func TestNew(t *testing.T) {
	type testCase struct {
		modify   func(*Config)
		err      string
		validate func(t *testing.T, ac *AutoConfig)
	}

	cases := map[string]testCase{
		"no-direct-rpc": {
			modify: func(c *Config) {
				c.DirectRPC = nil
			},
			err: "must provide a direct RPC delegate",
		},
		"no-config-loader": {
			modify: func(c *Config) {
				c.Loader = nil
			},
			err: "must provide a config loader",
		},
		"no-cache": {
			modify: func(c *Config) {
				c.Cache = nil
			},
			err: "must provide a cache",
		},
		"no-tls-configurator": {
			modify: func(c *Config) {
				c.TLSConfigurator = nil
			},
			err: "must provide a TLS configurator",
		},
		"no-tokens": {
			modify: func(c *Config) {
				c.Tokens = nil
			},
			err: "must provide a token store",
		},
		"ok": {
			validate: func(t *testing.T, ac *AutoConfig) {
				t.Helper()
				require.NotNil(t, ac.logger)
				require.NotNil(t, ac.acConfig.Waiter)
				require.Equal(t, time.Minute, ac.acConfig.FallbackRetry)
				require.Equal(t, 10*time.Second, ac.acConfig.FallbackLeeway)
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := Config{
				Loader: func(source config.Source) (result config.LoadResult, err error) {
					return config.LoadResult{}, nil
				},
				DirectRPC:        newMockDirectRPC(t),
				Tokens:           newMockTokenStore(t),
				Cache:            newMockCache(t),
				TLSConfigurator:  newMockTLSConfigurator(t),
				ServerProvider:   newMockServerProvider(t),
				EnterpriseConfig: newEnterpriseConfig(t),
			}

			if tcase.modify != nil {
				tcase.modify(&cfg)
			}

			ac, err := New(cfg)
			if tcase.err != "" {
				testutil.RequireErrorContains(t, err, tcase.err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, ac)
				if tcase.validate != nil {
					tcase.validate(t, ac)
				}
			}
		})
	}
}

func TestReadConfig(t *testing.T) {
	// just testing that some auto config source gets injected
	ac := AutoConfig{
		autoConfigSource: config.LiteralSource{
			Name:   autoConfigFileName,
			Config: config.Config{NodeName: stringPointer("hobbiton")},
		},
		logger: testutil.Logger(t),
		acConfig: Config{
			Loader: func(source config.Source) (config.LoadResult, error) {
				r := config.LoadResult{}
				cfg, _, err := source.Parse()
				if err != nil {
					return r, err
				}

				r.RuntimeConfig = &config.RuntimeConfig{
					DevMode:  true,
					NodeName: *cfg.NodeName,
				}
				return r, nil
			},
		},
	}

	cfg, err := ac.ReadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "hobbiton", cfg.NodeName)
	require.True(t, cfg.DevMode)
	require.Same(t, ac.config, cfg)
}

func setupRuntimeConfig(t *testing.T) *configLoader {
	t.Helper()

	dataDir := testutil.TempDir(t, "auto-config")

	opts := config.LoadOpts{
		FlagValues: config.FlagValuesTarget{
			Config: config.Config{
				DataDir:    &dataDir,
				Datacenter: stringPointer("dc1"),
				NodeName:   stringPointer("autoconf"),
				BindAddr:   stringPointer("127.0.0.1"),
			},
		},
	}
	return &configLoader{opts: opts}
}

func TestInitialConfiguration_disabled(t *testing.T) {
	mcfg := newMockedConfig(t)
	mcfg.loader.addConfigHCL(`
		primary_datacenter = "primary"
		auto_config = {
			enabled = false
		}
	`)

	ac, err := New(mcfg.Config)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)
	require.NoFileExists(t, filepath.Join(*mcfg.loader.opts.FlagValues.DataDir, autoConfigFileName))
}

func TestInitialConfiguration_cancelled(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	mcfg := newMockedConfig(t)

	loader := setupRuntimeConfig(t)
	loader.addConfigHCL(`
		primary_datacenter = "primary"
		auto_config = {
			enabled = true
			intro_token = "blarg"
			server_addresses = ["127.0.0.1:8300"]
		}
		verify_outgoing = true
	`)
	mcfg.Config.Loader = loader.Load

	expectedRequest := pbautoconf.AutoConfigRequest{
		Datacenter: "dc1",
		Node:       "autoconf",
		JWT:        "blarg",
	}

	mcfg.directRPC.On("RPC", "dc1", "autoconf", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8300}, "AutoConfig.InitialConfiguration", &expectedRequest, mock.Anything).Return(fmt.Errorf("injected error")).Times(0).Maybe()
	mcfg.serverProvider.On("FindLANServer").Return(nil).Times(0).Maybe()

	ac, err := New(mcfg.Config)
	require.NoError(t, err)
	require.NotNil(t, ac)

	ctx, cancelFn := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer cancelFn()

	cfg, err := ac.InitialConfiguration(ctx)
	testutil.RequireErrorContains(t, err, context.DeadlineExceeded.Error())
	require.Nil(t, cfg)
}

func TestInitialConfiguration_restored(t *testing.T) {
	mcfg := newMockedConfig(t)

	loader := setupRuntimeConfig(t)
	loader.addConfigHCL(`
		auto_config = {
			enabled = true
			intro_token ="blarg"
			server_addresses = ["127.0.0.1:8300"]
		}
		verify_outgoing = true
	`)

	mcfg.Config.Loader = loader.Load

	indexedRoots, cert, extraCACerts := mcfg.setupInitialTLS(t, "autoconf", "dc1", "secret")

	// persist an auto config response to the data dir where it is expected
	persistedFile := filepath.Join(*loader.opts.FlagValues.DataDir, autoConfigFileName)
	response := &pbautoconf.AutoConfigResponse{
		Config: &pbconfig.Config{
			PrimaryDatacenter: "primary",
			TLS: &pbconfig.TLS{
				VerifyServerHostname: true,
			},
			ACL: &pbconfig.ACL{
				Tokens: &pbconfig.ACLTokens{
					Agent: "secret",
				},
			},
		},
		CARoots:             mustTranslateCARootsToProtobuf(t, indexedRoots),
		Certificate:         mustTranslateIssuedCertToProtobuf(t, cert),
		ExtraCACertificates: extraCACerts,
	}
	data, err := pbMarshaler.Marshal(response)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(persistedFile, data, 0600))

	// recording the initial configuration even when restoring is going to update
	// the agent token in the token store
	mcfg.tokens.On("UpdateAgentToken", "secret", token.TokenSourceConfig).Return(true).Once()

	// prepopulation is going to grab the token to populate the correct cache key
	mcfg.tokens.On("AgentToken").Return("secret").Times(0)

	ac, err := New(mcfg.Config)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err, data)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)
}

func TestInitialConfiguration_success(t *testing.T) {
	mcfg := newMockedConfig(t)
	loader := setupRuntimeConfig(t)
	loader.addConfigHCL(`
		auto_config = {
			enabled = true
			intro_token ="blarg"
			server_addresses = ["127.0.0.1:8300"]
		}
		verify_outgoing = true
	`)
	mcfg.Config.Loader = loader.Load

	indexedRoots, cert, extraCerts := mcfg.setupInitialTLS(t, "autoconf", "dc1", "secret")

	// this gets called when InitialConfiguration is invoked to record the token from the
	// auto-config response
	mcfg.tokens.On("UpdateAgentToken", "secret", token.TokenSourceConfig).Return(true).Once()

	// prepopulation is going to grab the token to populate the correct cache key
	mcfg.tokens.On("AgentToken").Return("secret").Times(0)

	// no server provider
	mcfg.serverProvider.On("FindLANServer").Return(nil).Times(0)

	populateResponse := func(args mock.Arguments) {
		resp, ok := args.Get(5).(*pbautoconf.AutoConfigResponse)
		require.True(t, ok)
		resp.Config = &pbconfig.Config{
			PrimaryDatacenter: "primary",
			TLS: &pbconfig.TLS{
				VerifyServerHostname: true,
			},
			ACL: &pbconfig.ACL{
				Tokens: &pbconfig.ACLTokens{
					Agent: "secret",
				},
			},
		}

		resp.CARoots = mustTranslateCARootsToProtobuf(t, indexedRoots)
		resp.Certificate = mustTranslateIssuedCertToProtobuf(t, cert)
		resp.ExtraCACertificates = extraCerts
	}

	expectedRequest := pbautoconf.AutoConfigRequest{
		Datacenter: "dc1",
		Node:       "autoconf",
		JWT:        "blarg",
	}

	mcfg.directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8300},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(nil).Run(populateResponse)

	ac, err := New(mcfg.Config)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)

	// the file was written to.
	persistedFile := filepath.Join(*loader.opts.FlagValues.DataDir, autoConfigFileName)
	require.FileExists(t, persistedFile)
}

func TestInitialConfiguration_retries(t *testing.T) {
	mcfg := newMockedConfig(t)
	loader := setupRuntimeConfig(t)
	loader.addConfigHCL(`
		auto_config = {
			enabled = true
			intro_token ="blarg"
			server_addresses = [
				"198.18.0.1:8300",
				"198.18.0.2:8398",
				"198.18.0.3:8399",
				"127.0.0.1:1234"
			]
		}
		verify_outgoing = true
	`)
	mcfg.Config.Loader = loader.Load

	// reduce the retry wait times to make this test run faster
	mcfg.Config.Waiter = &retry.Waiter{MinFailures: 2, MaxWait: time.Millisecond}

	indexedRoots, cert, extraCerts := mcfg.setupInitialTLS(t, "autoconf", "dc1", "secret")

	// this gets called when InitialConfiguration is invoked to record the token from the
	// auto-config response
	mcfg.tokens.On("UpdateAgentToken", "secret", token.TokenSourceConfig).Return(true).Once()

	// prepopulation is going to grab the token to populate the correct cache key
	mcfg.tokens.On("AgentToken").Return("secret").Times(0)

	// no server provider
	mcfg.serverProvider.On("FindLANServer").Return(nil).Times(0)

	populateResponse := func(args mock.Arguments) {
		resp, ok := args.Get(5).(*pbautoconf.AutoConfigResponse)
		require.True(t, ok)
		resp.Config = &pbconfig.Config{
			PrimaryDatacenter: "primary",
			TLS: &pbconfig.TLS{
				VerifyServerHostname: true,
			},
			ACL: &pbconfig.ACL{
				Tokens: &pbconfig.ACLTokens{
					Agent: "secret",
				},
			},
		}

		resp.CARoots = mustTranslateCARootsToProtobuf(t, indexedRoots)
		resp.Certificate = mustTranslateIssuedCertToProtobuf(t, cert)
		resp.ExtraCACertificates = extraCerts
	}

	expectedRequest := pbautoconf.AutoConfigRequest{
		Datacenter: "dc1",
		Node:       "autoconf",
		JWT:        "blarg",
	}

	// basically the 198.18.0.* addresses should fail indefinitely. the first time through the
	// outer loop we inject a failure for the DNS resolution of localhost to 127.0.0.1. Then
	// the second time through the outer loop we allow the localhost one to work.
	mcfg.directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 8300},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(fmt.Errorf("injected failure")).Times(0)
	mcfg.directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(198, 18, 0, 2), Port: 8398},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(fmt.Errorf("injected failure")).Times(0)
	mcfg.directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(198, 18, 0, 3), Port: 8399},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(fmt.Errorf("injected failure")).Times(0)
	mcfg.directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(fmt.Errorf("injected failure")).Once()
	mcfg.directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(nil).Run(populateResponse).Once()

	ac, err := New(mcfg.Config)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)

	// the file was written to.
	persistedFile := filepath.Join(*loader.opts.FlagValues.DataDir, autoConfigFileName)
	require.FileExists(t, persistedFile)
}

func TestGoRoutineManagement(t *testing.T) {
	mcfg := newMockedConfig(t)
	loader := setupRuntimeConfig(t)
	loader.addConfigHCL(`
		auto_config = {
			enabled = true
			intro_token ="blarg"
			server_addresses = ["127.0.0.1:8300"]
		}
		verify_outgoing = true
	`)
	mcfg.Config.Loader = loader.Load

	// prepopulation is going to grab the token to populate the correct cache key
	mcfg.tokens.On("AgentToken").Return("secret").Times(0)

	ac, err := New(mcfg.Config)
	require.NoError(t, err)

	// priming the config so some other requests will work properly that need to
	// read from the configuration. We are going to avoid doing InitialConfiguration
	// for this test as we only are really concerned with the go routine management
	_, err = ac.ReadConfig()
	require.NoError(t, err)

	var rootsCtx context.Context
	var leafCtx context.Context
	var ctxLock sync.Mutex

	rootsReq := ac.caRootsRequest()
	mcfg.cache.On("Notify",
		mock.Anything,
		cachetype.ConnectCARootName,
		&rootsReq,
		rootsWatchID,
		mock.Anything,
	).Return(nil).Times(2).Run(func(args mock.Arguments) {
		ctxLock.Lock()
		rootsCtx = args.Get(0).(context.Context)
		ctxLock.Unlock()
	})

	leafReq := ac.leafCertRequest()
	mcfg.leafCerts.On("Notify",
		mock.Anything,
		&leafReq,
		leafWatchID,
		mock.Anything,
	).Return(nil).Times(2).Run(func(args mock.Arguments) {
		ctxLock.Lock()
		leafCtx = args.Get(0).(context.Context)
		ctxLock.Unlock()
	})

	// we will start/stop things twice
	mcfg.tokens.On("Notify", token.TokenKindAgent).Return(token.Notifier{}).Times(2)
	mcfg.tokens.On("StopNotify", token.Notifier{}).Times(2)

	mcfg.tlsCfg.On("AutoEncryptCert").Return(&x509.Certificate{
		NotAfter: time.Now().Add(10 * time.Minute),
	}).Times(0)

	// ensure that auto-config isn't running
	require.False(t, ac.IsRunning())

	// ensure that nothing bad happens and that it reports as stopped
	require.False(t, ac.Stop())

	// ensure that the Done chan also reports that things are not running
	// in other words the chan is immediately selectable
	requireChanReady(t, ac.Done())

	// start auto-config
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, ac.Start(ctx))

	waitForContexts := func() bool {
		ctxLock.Lock()
		defer ctxLock.Unlock()
		return !(rootsCtx == nil || leafCtx == nil)
	}

	// wait for the cache notifications to get started
	require.Eventually(t, waitForContexts, 100*time.Millisecond, 10*time.Millisecond)

	// hold onto the Done chan to test for the go routine exiting
	done := ac.Done()

	// ensure we report as running
	require.True(t, ac.IsRunning())

	// ensure the done chan is not selectable yet
	requireChanNotReady(t, done)

	// ensure we error if we attempt to start again
	err = ac.Start(ctx)
	testutil.RequireErrorContains(t, err, "AutoConfig is already running")

	// now stop things - it should return true indicating that it was running
	// when we attempted to stop it.
	require.True(t, ac.Stop())

	// ensure that the go routine shuts down - it will close the done chan. Also it should cancel
	// the cache watches by cancelling the context it passed into the Notify call.
	require.True(t, waitForChans(100*time.Millisecond, done, leafCtx.Done(), rootsCtx.Done()), "AutoConfig didn't shut down")
	require.False(t, ac.IsRunning())

	// restart it
	require.NoError(t, ac.Start(ctx))

	// get the new Done chan
	done = ac.Done()

	// ensure that context cancellation causes us to stop as well
	cancel()
	require.True(t, waitForChans(100*time.Millisecond, done))
}

type testAutoConfig struct {
	mcfg          *mockedConfig
	ac            *AutoConfig
	tokenUpdates  chan struct{}
	originalToken string
	stop          func()

	initialRoots *structs.IndexedCARoots
	initialCert  *structs.IssuedCert
	extraCerts   []string
}

func startedAutoConfig(t *testing.T, autoEncrypt bool) testAutoConfig {
	t.Helper()
	mcfg := newMockedConfig(t)
	loader := setupRuntimeConfig(t)
	if !autoEncrypt {
		loader.addConfigHCL(`
			auto_config = {
				enabled = true
				intro_token ="blarg"
				server_addresses = ["127.0.0.1:8300"]
			}
			verify_outgoing = true
		`)
	} else {
		loader.addConfigHCL(`
			auto_encrypt {
				tls = true
			}
			verify_outgoing = true
		`)
	}
	mcfg.Config.Loader = loader.Load
	mcfg.Config.FallbackLeeway = time.Nanosecond

	originalToken := "a5deaa25-11ca-48bf-a979-4c3a7aa4b9a9"

	if !autoEncrypt {
		// this gets called when InitialConfiguration is invoked to record the token from the
		// auto-config response
		mcfg.tokens.On("UpdateAgentToken", originalToken, token.TokenSourceConfig).Return(true).Once()
	}

	// we expect this to be retrieved twice: first during cache prepopulation
	// and then again when setting up the cache watch for the leaf cert.
	// However one of those expectations is setup in the expectInitialTLS
	// method so we only need one more here
	mcfg.tokens.On("AgentToken").Return(originalToken).Once()

	if autoEncrypt {
		// when using AutoEncrypt we also have to grab the token once more
		// when setting up the initial RPC as the ACL token is what is used
		// to authorize the request.
		mcfg.tokens.On("AgentToken").Return(originalToken).Once()
	}

	// this is called once during Start to initialze the token watches
	tokenUpdateCh := make(chan struct{})
	tokenNotifier := token.Notifier{
		Ch: tokenUpdateCh,
	}
	mcfg.tokens.On("Notify", token.TokenKindAgent).Once().Return(tokenNotifier)
	mcfg.tokens.On("StopNotify", tokenNotifier).Once()

	// expect the roots watch on the cache
	mcfg.cache.On("Notify",
		mock.Anything,
		cachetype.ConnectCARootName,
		&structs.DCSpecificRequest{Datacenter: "dc1"},
		rootsWatchID,
		mock.Anything,
	).Return(nil).Once()

	mcfg.leafCerts.On("Notify",
		mock.Anything,
		&leafcert.ConnectCALeafRequest{
			Datacenter: "dc1",
			Agent:      "autoconf",
			Token:      originalToken,
			DNSSAN:     defaultDNSSANs,
			IPSAN:      defaultIPSANs,
		},
		leafWatchID,
		mock.Anything,
	).Return(nil).Once()

	// override the server provider - most of the other tests set it up so that this
	// always returns no server (simulating a state where we haven't joined gossip).
	// this seems like a good place to ensure this other way of finding server information
	// works
	mcfg.serverProvider.On("FindLANServer").Once().Return(&metadata.Server{
		Addr: &net.TCPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 8300},
	})

	indexedRoots, cert, extraCerts := mcfg.setupInitialTLS(t, "autoconf", "dc1", originalToken)

	mcfg.tlsCfg.On("AutoEncryptCert").Return(&x509.Certificate{
		NotAfter: cert.ValidBefore,
	}).Once()

	populateResponse := func(args mock.Arguments) {
		method := args.String(3)

		switch method {
		case "AutoConfig.InitialConfiguration":
			resp, ok := args.Get(5).(*pbautoconf.AutoConfigResponse)
			require.True(t, ok)
			resp.Config = &pbconfig.Config{
				PrimaryDatacenter: "primary",
				TLS: &pbconfig.TLS{
					VerifyServerHostname: true,
				},
				ACL: &pbconfig.ACL{
					Tokens: &pbconfig.ACLTokens{
						Agent: originalToken,
					},
				},
			}

			resp.CARoots = mustTranslateCARootsToProtobuf(t, indexedRoots)
			resp.Certificate = mustTranslateIssuedCertToProtobuf(t, cert)
			resp.ExtraCACertificates = extraCerts
		case "AutoEncrypt.Sign":
			resp, ok := args.Get(5).(*structs.SignedResponse)
			require.True(t, ok)
			*resp = structs.SignedResponse{
				VerifyServerHostname: true,
				ConnectCARoots:       *indexedRoots,
				IssuedCert:           *cert,
				ManualCARoots:        extraCerts,
			}
		}
	}

	if !autoEncrypt {
		expectedRequest := pbautoconf.AutoConfigRequest{
			Datacenter: "dc1",
			Node:       "autoconf",
			JWT:        "blarg",
		}

		mcfg.directRPC.On(
			"RPC",
			"dc1",
			"autoconf",
			&net.TCPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 8300},
			"AutoConfig.InitialConfiguration",
			&expectedRequest,
			&pbautoconf.AutoConfigResponse{}).Return(nil).Run(populateResponse).Once()
	} else {
		expectedRequest := structs.CASignRequest{
			WriteRequest: structs.WriteRequest{Token: originalToken},
			Datacenter:   "dc1",
			// TODO (autoconf) Maybe in the future we should populate a CSR
			// and do some manual parsing/verification of the contents. The
			// bits not having to do with the signing key such as the requested
			// SANs and CN. For now though the mockDirectRPC type will empty
			// the CSR so we have to pass in an empty string to the expectation.
			CSR: "",
		}

		mcfg.directRPC.On(
			"RPC",
			"dc1",
			"autoconf", // reusing the same name to prevent needing more configurability
			&net.TCPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 8300},
			"AutoEncrypt.Sign",
			&expectedRequest,
			&structs.SignedResponse{}).Return(nil).Run(populateResponse)
	}

	ac, err := New(mcfg.Config)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	if !autoEncrypt {
		// auto-encrypt doesn't modify the config but rather sets the value
		// in the TLS configurator
		require.True(t, cfg.TLS.InternalRPC.VerifyServerHostname)
	}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ac.Start(ctx))
	t.Cleanup(func() {
		done := ac.Done()
		cancel()
		timer := time.NewTimer(1 * time.Second)
		defer timer.Stop()
		select {
		case <-done:
			// do nothing
		case <-timer.C:
			t.Fatalf("AutoConfig wasn't stopped within 1 second after test completion")
		}
	})

	return testAutoConfig{
		mcfg:          mcfg,
		ac:            ac,
		tokenUpdates:  tokenUpdateCh,
		originalToken: originalToken,
		initialRoots:  indexedRoots,
		initialCert:   cert,
		extraCerts:    extraCerts,
		stop:          cancel,
	}
}

// this test ensures that the cache watches are restarted with
// the updated token after receiving a token update
func TestTokenUpdate(t *testing.T) {
	testAC := startedAutoConfig(t, false)

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
	testAC.mcfg.leafCerts.On("Notify",
		mock.Anything,
		&leafcert.ConnectCALeafRequest{
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

func TestRootsUpdate(t *testing.T) {
	testAC := startedAutoConfig(t, false)

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
	testAC.mcfg.tlsCfg.On("UpdateAutoTLS",
		testAC.extraCerts,
		[]string{secondCA.RootCert, testAC.initialRoots.Roots[0].RootCert},
		testAC.initialCert.CertPEM,
		"redacted",
		true,
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

	// persisting these to disk happens right after the chan we are waiting for will have fired above
	// however there is no deterministic way to know once its been written outside of maybe a filesystem
	// event notifier. That seems a little heavy handed just for this and especially to do in any sort
	// of cross platform way.
	testretry.Run(t, func(r *testretry.R) {
		resp, err := testAC.ac.readPersistedAutoConfig()
		require.NoError(r, err)
		require.Equal(r, secondRoots.ActiveRootID, resp.CARoots.GetActiveRootID())
	})
}

func TestCertUpdate(t *testing.T) {
	testAC := startedAutoConfig(t, false)
	secondCert := newLeaf(t, "autoconf", "dc1", testAC.initialRoots.Roots[0], 99, 10*time.Minute)

	updatedCtx, cancel := context.WithCancel(context.Background())
	testAC.mcfg.tlsCfg.On("UpdateAutoTLS",
		testAC.extraCerts,
		[]string{testAC.initialRoots.Roots[0].RootCert},
		secondCert.CertPEM,
		"redacted",
		true,
	).Return(nil).Once().Run(func(args mock.Arguments) {
		cancel()
	})

	// when a cache event comes in we end up recalculating the fallback timer which requires this call
	testAC.mcfg.tlsCfg.On("AutoEncryptCert").Return(&x509.Certificate{
		NotAfter: secondCert.ValidBefore,
	}).Once()

	req := leafcert.ConnectCALeafRequest{
		Datacenter: "dc1",
		Agent:      "autoconf",
		Token:      testAC.originalToken,
		DNSSAN:     defaultDNSSANs,
		IPSAN:      defaultIPSANs,
	}
	require.True(t, testAC.mcfg.leafCerts.sendNotification(context.Background(), req.Key(), cache.UpdateEvent{
		CorrelationID: leafWatchID,
		Result:        secondCert,
		Meta: cache.ResultMeta{
			Index: secondCert.ModifyIndex,
		},
	}))

	require.True(t, waitForChans(100*time.Millisecond, updatedCtx.Done()), "TLS certificates were not updated within the alotted time")

	// persisting these to disk happens after all the things we would wait for in assertCertUpdated
	// will have fired. There is no deterministic way to know once its been written so we wrap
	// this in a retry.
	testretry.Run(t, func(r *testretry.R) {
		resp, err := testAC.ac.readPersistedAutoConfig()
		require.NoError(r, err)

		// ensure the roots got persisted to disk
		require.Equal(r, secondCert.CertPEM, resp.Certificate.GetCertPEM())
	})
}

func TestFallback(t *testing.T) {
	testAC := startedAutoConfig(t, false)

	// at this point everything is operating normally and we are just
	// waiting for events. We are going to send a new cert that is basically
	// already expired and then allow the fallback routine to kick in.
	secondCert := newLeaf(t, "autoconf", "dc1", testAC.initialRoots.Roots[0], 100, time.Nanosecond)
	secondCA := caRootRoundtrip(t, connect.TestCA(t, testAC.initialRoots.Roots[0]))
	secondRoots := caRootsRoundtrip(t, &structs.IndexedCARoots{
		ActiveRootID: secondCA.ID,
		TrustDomain:  connect.TestClusterID,
		Roots: []*structs.CARoot{
			secondCA,
			testAC.initialRoots.Roots[0],
		},
		QueryMeta: structs.QueryMeta{
			Index: 101,
		},
	})
	thirdCert := newLeaf(t, "autoconf", "dc1", secondCA, 102, 10*time.Minute)

	// setup the expectation for when the certs got updated initially
	updatedCtx, updateCancel := context.WithCancel(context.Background())
	testAC.mcfg.tlsCfg.On("UpdateAutoTLS",
		testAC.extraCerts,
		[]string{testAC.initialRoots.Roots[0].RootCert},
		secondCert.CertPEM,
		"redacted",
		true,
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
		resp, ok := args.Get(5).(*pbautoconf.AutoConfigResponse)
		require.True(t, ok)
		resp.Config = &pbconfig.Config{
			PrimaryDatacenter: "primary",
			TLS: &pbconfig.TLS{
				VerifyServerHostname: true,
			},
			ACL: &pbconfig.ACL{
				Tokens: &pbconfig.ACLTokens{
					Agent: testAC.originalToken,
				},
			},
		}

		resp.CARoots = mustTranslateCARootsToProtobuf(t, secondRoots)
		resp.Certificate = mustTranslateIssuedCertToProtobuf(t, thirdCert)
		resp.ExtraCACertificates = testAC.extraCerts

		fallbackCancel()
	}

	expectedRequest := pbautoconf.AutoConfigRequest{
		Datacenter: "dc1",
		Node:       "autoconf",
		JWT:        "blarg",
	}

	testAC.mcfg.directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(198, 18, 23, 2), Port: 8300},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(nil).Run(populateResponse).Once()

	// this gets called when InitialConfiguration is invoked to record the token from the
	// auto-config response which is how the Fallback for auto-config works
	testAC.mcfg.tokens.On("UpdateAgentToken", testAC.originalToken, token.TokenSourceConfig).Return(true).Once()

	testAC.mcfg.expectInitialTLS(t, "autoconf", "dc1", testAC.originalToken, secondCA, secondRoots, thirdCert, testAC.extraCerts)

	// after the second RPC we now will use the new certs validity period in the next run loop iteration
	testAC.mcfg.tlsCfg.On("AutoEncryptCert").Return(&x509.Certificate{
		NotAfter: time.Now().Add(10 * time.Minute),
	}).Once()

	// now that all the mocks are set up we can trigger the whole thing by sending the second expired cert
	// as a cache update event.
	req := leafcert.ConnectCALeafRequest{
		Datacenter: "dc1",
		Agent:      "autoconf",
		Token:      testAC.originalToken,
		DNSSAN:     defaultDNSSANs,
		IPSAN:      defaultIPSANs,
	}
	require.True(t, testAC.mcfg.leafCerts.sendNotification(context.Background(), req.Key(), cache.UpdateEvent{
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

	testAC.stop()
	<-testAC.ac.done

	resp, err := testAC.ac.readPersistedAutoConfig()
	require.NoError(t, err)

	// ensure the roots got persisted to disk
	require.Equal(t, thirdCert.CertPEM, resp.Certificate.GetCertPEM())
	require.Equal(t, secondRoots.ActiveRootID, resp.CARoots.GetActiveRootID())
}

func TestIntroToken(t *testing.T) {
	tokenFile := testutil.TempFile(t, "intro-token")
	t.Cleanup(func() { os.Remove(tokenFile.Name()) })

	tokenFileEmpty := testutil.TempFile(t, "intro-token-empty")
	t.Cleanup(func() { os.Remove(tokenFileEmpty.Name()) })

	tokenFromFile := "8ae34d3a-8adf-446a-b236-69874597cb5b"
	tokenFromConfig := "3ad9b572-ea42-4e47-9cd0-53a398a98abf"
	require.NoError(t, os.WriteFile(tokenFile.Name(), []byte(tokenFromFile), 0600))

	type testCase struct {
		config *config.RuntimeConfig
		err    string
		token  string
	}

	cases := map[string]testCase{
		"config": {
			config: &config.RuntimeConfig{
				AutoConfig: config.AutoConfig{
					IntroToken:     tokenFromConfig,
					IntroTokenFile: tokenFile.Name(),
				},
			},
			token: tokenFromConfig,
		},
		"file": {
			config: &config.RuntimeConfig{
				AutoConfig: config.AutoConfig{
					IntroTokenFile: tokenFile.Name(),
				},
			},
			token: tokenFromFile,
		},
		"file-empty": {
			config: &config.RuntimeConfig{
				AutoConfig: config.AutoConfig{
					IntroTokenFile: tokenFileEmpty.Name(),
				},
			},
			err: "intro_token_file did not contain any token",
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ac := AutoConfig{
				config: tcase.config,
			}

			token, err := ac.introToken()
			if tcase.err != "" {
				testutil.RequireErrorContains(t, err, tcase.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tcase.token, token)
			}
		})
	}

}
