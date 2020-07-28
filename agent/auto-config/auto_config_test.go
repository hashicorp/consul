package autoconf

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/types"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbautoconf"
	"github.com/hashicorp/consul/proto/pbconfig"
	"github.com/hashicorp/consul/proto/pbconnect"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockDirectRPC struct {
	mock.Mock
}

func (m *mockDirectRPC) RPC(dc string, node string, addr net.Addr, method string, args interface{}, reply interface{}) error {
	var retValues mock.Arguments
	if method == "AutoConfig.InitialConfiguration" {
		req := args.(*pbautoconf.AutoConfigRequest)
		csr := req.CSR
		req.CSR = ""
		retValues = m.Called(dc, node, addr, method, args, reply)
		req.CSR = csr
	} else {
		retValues = m.Called(dc, node, addr, method, args, reply)
	}

	switch ret := retValues.Get(0).(type) {
	case error:
		return ret
	case func(interface{}):
		ret(reply)
		return nil
	default:
		return fmt.Errorf("This should not happen, update mock direct rpc expectations")
	}
}

type mockCertMonitor struct {
	mock.Mock
}

func (m *mockCertMonitor) Start(_ context.Context) (<-chan struct{}, error) {
	ret := m.Called()
	ch := ret.Get(0).(<-chan struct{})
	return ch, ret.Error(1)
}

func (m *mockCertMonitor) Stop() bool {
	return m.Called().Bool(0)
}

func (m *mockCertMonitor) Update(resp *structs.SignedResponse) error {
	var privKey string
	// filter out real certificates as we cannot predict their values
	if resp != nil && strings.HasPrefix(resp.IssuedCert.PrivateKeyPEM, "-----BEGIN") {
		privKey = resp.IssuedCert.PrivateKeyPEM
		resp.IssuedCert.PrivateKeyPEM = ""
	}
	err := m.Called(resp).Error(0)
	if privKey != "" {
		resp.IssuedCert.PrivateKeyPEM = privKey
	}
	return err
}

func TestNew(t *testing.T) {
	type testCase struct {
		config   Config
		err      string
		validate func(t *testing.T, ac *AutoConfig)
	}

	cases := map[string]testCase{
		"no-direct-rpc": {
			err: "must provide a direct RPC delegate",
		},
		"ok": {
			config: Config{
				DirectRPC: &mockDirectRPC{},
			},
			validate: func(t *testing.T, ac *AutoConfig) {
				t.Helper()
				require.NotNil(t, ac.logger)
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ac, err := New(&tcase.config)
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

func TestLoadConfig(t *testing.T) {
	// Basically just testing that injection of the extra
	// source works.
	devMode := true
	builderOpts := config.BuilderOpts{
		// putting this in dev mode so that the config validates
		// without having to specify a data directory
		DevMode: &devMode,
	}

	cfg, warnings, err := LoadConfig(builderOpts, config.Source{
		Name:   "test",
		Format: "hcl",
		Data:   `node_name = "hobbiton"`,
	},
		config.Source{
			Name:   "overrides",
			Format: "json",
			Data:   `{"check_reap_interval": "1ms"}`,
		})

	require.NoError(t, err)
	require.Empty(t, warnings)
	require.NotNil(t, cfg)
	require.Equal(t, "hobbiton", cfg.NodeName)
	require.Equal(t, 1*time.Millisecond, cfg.CheckReapInterval)
}

func TestReadConfig(t *testing.T) {
	// just testing that some auto config source gets injected
	devMode := true
	ac := AutoConfig{
		autoConfigData: `{"node_name": "hobbiton"}`,
		builderOpts: config.BuilderOpts{
			// putting this in dev mode so that the config validates
			// without having to specify a data directory
			DevMode: &devMode,
		},
		logger: testutil.Logger(t),
	}

	cfg, err := ac.ReadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "hobbiton", cfg.NodeName)
	require.Same(t, ac.config, cfg)
}

func testSetupAutoConf(t *testing.T) (string, string, config.BuilderOpts) {
	t.Helper()

	// create top level directory to hold both config and data
	tld := testutil.TempDir(t, "auto-config")
	t.Cleanup(func() { os.RemoveAll(tld) })

	// create the data directory
	dataDir := filepath.Join(tld, "data")
	require.NoError(t, os.Mkdir(dataDir, 0700))

	// create the config directory
	configDir := filepath.Join(tld, "config")
	require.NoError(t, os.Mkdir(configDir, 0700))

	builderOpts := config.BuilderOpts{
		HCL: []string{
			`data_dir = "` + dataDir + `"`,
			`datacenter = "dc1"`,
			`node_name = "autoconf"`,
			`bind_addr = "127.0.0.1"`,
		},
	}

	return dataDir, configDir, builderOpts
}

func TestInitialConfiguration_disabled(t *testing.T) {
	dataDir, configDir, builderOpts := testSetupAutoConf(t)

	cfgFile := filepath.Join(configDir, "test.json")
	require.NoError(t, ioutil.WriteFile(cfgFile, []byte(`{
		"primary_datacenter": "primary", 
		"auto_config": {"enabled": false}
	}`), 0600))

	builderOpts.ConfigFiles = append(builderOpts.ConfigFiles, cfgFile)

	directRPC := mockDirectRPC{}
	conf := new(Config).
		WithBuilderOpts(builderOpts).
		WithDirectRPC(&directRPC)
	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)
	require.NoFileExists(t, filepath.Join(dataDir, autoConfigFileName))

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
}

func TestInitialConfiguration_cancelled(t *testing.T) {
	_, configDir, builderOpts := testSetupAutoConf(t)

	cfgFile := filepath.Join(configDir, "test.json")
	require.NoError(t, ioutil.WriteFile(cfgFile, []byte(`{
		"primary_datacenter": "primary", 
		"auto_config": {"enabled": true, "intro_token": "blarg", "server_addresses": ["127.0.0.1:8300"]},
		"verify_outgoing": true
	}`), 0600))

	builderOpts.ConfigFiles = append(builderOpts.ConfigFiles, cfgFile)

	directRPC := mockDirectRPC{}

	expectedRequest := pbautoconf.AutoConfigRequest{
		Datacenter: "dc1",
		Node:       "autoconf",
		JWT:        "blarg",
	}

	directRPC.On("RPC", "dc1", "autoconf", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8300}, "AutoConfig.InitialConfiguration", &expectedRequest, mock.Anything).Return(fmt.Errorf("injected error")).Times(0)
	conf := new(Config).
		WithBuilderOpts(builderOpts).
		WithDirectRPC(&directRPC)
	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)

	ctx, cancelFn := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer cancelFn()

	cfg, err := ac.InitialConfiguration(ctx)
	testutil.RequireErrorContains(t, err, context.DeadlineExceeded.Error())
	require.Nil(t, cfg)

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
}

func TestInitialConfiguration_restored(t *testing.T) {
	dataDir, configDir, builderOpts := testSetupAutoConf(t)

	cfgFile := filepath.Join(configDir, "test.json")
	require.NoError(t, ioutil.WriteFile(cfgFile, []byte(`{
		"auto_config": {"enabled": true, "intro_token": "blarg", "server_addresses": ["127.0.0.1:8300"]}, "verify_outgoing": true
	}`), 0600))

	builderOpts.ConfigFiles = append(builderOpts.ConfigFiles, cfgFile)

	// persist an auto config response to the data dir where it is expected
	persistedFile := filepath.Join(dataDir, autoConfigFileName)
	response := &pbautoconf.AutoConfigResponse{
		Config: &pbconfig.Config{
			PrimaryDatacenter: "primary",
			TLS: &pbconfig.TLS{
				VerifyServerHostname: true,
			},
		},
		CARoots: &pbconnect.CARoots{
			ActiveRootID: "active",
			TrustDomain:  "trust",
			Roots: []*pbconnect.CARoot{
				{
					ID:           "active",
					Name:         "foo",
					SerialNumber: 42,
					SigningKeyID: "blarg",
					NotBefore:    &types.Timestamp{Seconds: 5000, Nanos: 100},
					NotAfter:     &types.Timestamp{Seconds: 10000, Nanos: 9009},
					RootCert:     "not an actual cert",
					Active:       true,
				},
			},
		},
		Certificate: &pbconnect.IssuedCert{
			SerialNumber:  "1234",
			CertPEM:       "not a cert",
			PrivateKeyPEM: "private",
			Agent:         "foo",
			AgentURI:      "spiffe://blarg/agent/client/dc/foo/id/foo",
			ValidAfter:    &types.Timestamp{Seconds: 6000},
			ValidBefore:   &types.Timestamp{Seconds: 7000},
		},
		ExtraCACertificates: []string{"blarg"},
	}
	data, err := pbMarshaler.MarshalToString(response)
	require.NoError(t, err)
	require.NoError(t, ioutil.WriteFile(persistedFile, []byte(data), 0600))

	directRPC := mockDirectRPC{}

	// setup the mock certificate monitor to ensure that the initial state gets
	// updated appropriately during config restoration.
	certMon := mockCertMonitor{}
	certMon.On("Update", &structs.SignedResponse{
		IssuedCert: structs.IssuedCert{
			SerialNumber:  "1234",
			CertPEM:       "not a cert",
			PrivateKeyPEM: "private",
			Agent:         "foo",
			AgentURI:      "spiffe://blarg/agent/client/dc/foo/id/foo",
			ValidAfter:    time.Unix(6000, 0),
			ValidBefore:   time.Unix(7000, 0),
		},
		ConnectCARoots: structs.IndexedCARoots{
			ActiveRootID: "active",
			TrustDomain:  "trust",
			Roots: []*structs.CARoot{
				{
					ID:           "active",
					Name:         "foo",
					SerialNumber: 42,
					SigningKeyID: "blarg",
					NotBefore:    time.Unix(5000, 100),
					NotAfter:     time.Unix(10000, 9009),
					RootCert:     "not an actual cert",
					Active:       true,
					// the decoding process doesn't leave this nil
					IntermediateCerts: []string{},
				},
			},
		},
		ManualCARoots:        []string{"blarg"},
		VerifyServerHostname: true,
	}).Return(nil).Once()

	conf := new(Config).
		WithBuilderOpts(builderOpts).
		WithDirectRPC(&directRPC).
		WithCertMonitor(&certMon)
	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err, data)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
	certMon.AssertExpectations(t)
}

func TestInitialConfiguration_success(t *testing.T) {
	dataDir, configDir, builderOpts := testSetupAutoConf(t)

	cfgFile := filepath.Join(configDir, "test.json")
	require.NoError(t, ioutil.WriteFile(cfgFile, []byte(`{
		"auto_config": {"enabled": true, "intro_token": "blarg", "server_addresses": ["127.0.0.1:8300"]}, "verify_outgoing": true
	}`), 0600))

	builderOpts.ConfigFiles = append(builderOpts.ConfigFiles, cfgFile)

	persistedFile := filepath.Join(dataDir, autoConfigFileName)
	directRPC := mockDirectRPC{}

	populateResponse := func(val interface{}) {
		resp, ok := val.(*pbautoconf.AutoConfigResponse)
		require.True(t, ok)
		resp.Config = &pbconfig.Config{
			PrimaryDatacenter: "primary",
			TLS: &pbconfig.TLS{
				VerifyServerHostname: true,
			},
		}

		resp.CARoots = &pbconnect.CARoots{
			ActiveRootID: "active",
			TrustDomain:  "trust",
			Roots: []*pbconnect.CARoot{
				{
					ID:           "active",
					Name:         "foo",
					SerialNumber: 42,
					SigningKeyID: "blarg",
					NotBefore:    &types.Timestamp{Seconds: 5000, Nanos: 100},
					NotAfter:     &types.Timestamp{Seconds: 10000, Nanos: 9009},
					RootCert:     "not an actual cert",
					Active:       true,
				},
			},
		}
		resp.Certificate = &pbconnect.IssuedCert{
			SerialNumber: "1234",
			CertPEM:      "not a cert",
			Agent:        "foo",
			AgentURI:     "spiffe://blarg/agent/client/dc/foo/id/foo",
			ValidAfter:   &types.Timestamp{Seconds: 6000},
			ValidBefore:  &types.Timestamp{Seconds: 7000},
		}
		resp.ExtraCACertificates = []string{"blarg"}
	}

	expectedRequest := pbautoconf.AutoConfigRequest{
		Datacenter: "dc1",
		Node:       "autoconf",
		JWT:        "blarg",
	}

	directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8300},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(populateResponse)

	// setup the mock certificate monitor to ensure that the initial state gets
	// updated appropriately during config restoration.
	certMon := mockCertMonitor{}
	certMon.On("Update", &structs.SignedResponse{
		IssuedCert: structs.IssuedCert{
			SerialNumber:  "1234",
			CertPEM:       "not a cert",
			PrivateKeyPEM: "", // the mock
			Agent:         "foo",
			AgentURI:      "spiffe://blarg/agent/client/dc/foo/id/foo",
			ValidAfter:    time.Unix(6000, 0),
			ValidBefore:   time.Unix(7000, 0),
		},
		ConnectCARoots: structs.IndexedCARoots{
			ActiveRootID: "active",
			TrustDomain:  "trust",
			Roots: []*structs.CARoot{
				{
					ID:           "active",
					Name:         "foo",
					SerialNumber: 42,
					SigningKeyID: "blarg",
					NotBefore:    time.Unix(5000, 100),
					NotAfter:     time.Unix(10000, 9009),
					RootCert:     "not an actual cert",
					Active:       true,
				},
			},
		},
		ManualCARoots:        []string{"blarg"},
		VerifyServerHostname: true,
	}).Return(nil).Once()

	conf := new(Config).
		WithBuilderOpts(builderOpts).
		WithDirectRPC(&directRPC).
		WithCertMonitor(&certMon)
	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)

	// the file was written to.
	require.FileExists(t, persistedFile)

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
	certMon.AssertExpectations(t)
}

func TestInitialConfiguration_retries(t *testing.T) {
	dataDir, configDir, builderOpts := testSetupAutoConf(t)

	cfgFile := filepath.Join(configDir, "test.json")
	require.NoError(t, ioutil.WriteFile(cfgFile, []byte(`{
		"auto_config": {"enabled": true, "intro_token": "blarg", "server_addresses": ["198.18.0.1", "198.18.0.2:8398", "198.18.0.3:8399", "127.0.0.1:1234"]}, "verify_outgoing": true
	}`), 0600))

	builderOpts.ConfigFiles = append(builderOpts.ConfigFiles, cfgFile)

	persistedFile := filepath.Join(dataDir, autoConfigFileName)
	directRPC := mockDirectRPC{}

	populateResponse := func(val interface{}) {
		resp, ok := val.(*pbautoconf.AutoConfigResponse)
		require.True(t, ok)
		resp.Config = &pbconfig.Config{
			PrimaryDatacenter: "primary",
		}
	}

	expectedRequest := pbautoconf.AutoConfigRequest{
		Datacenter: "dc1",
		Node:       "autoconf",
		JWT:        "blarg",
	}

	// basically the 198.18.0.* addresses should fail indefinitely. the first time through the
	// outer loop we inject a failure for the DNS resolution of localhost to 127.0.0.1. Then
	// the second time through the outer loop we allow the localhost one to work.
	directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(198, 18, 0, 1), Port: 8300},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(fmt.Errorf("injected failure")).Times(0)
	directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(198, 18, 0, 2), Port: 8398},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(fmt.Errorf("injected failure")).Times(0)
	directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(198, 18, 0, 3), Port: 8399},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(fmt.Errorf("injected failure")).Times(0)
	directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(fmt.Errorf("injected failure")).Once()
	directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(populateResponse)

	waiter := lib.NewRetryWaiter(2, 0, 1*time.Millisecond, nil)
	conf := new(Config).
		WithBuilderOpts(builderOpts).
		WithDirectRPC(&directRPC).
		WithRetryWaiter(waiter)
	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)

	// the file was written to.
	require.FileExists(t, persistedFile)

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
}

func TestAutoConfig_StartStop(t *testing.T) {
	// currently the only thing running for autoconf is just the cert monitor
	// so this test only needs to ensure that the cert monitor is started and
	// stopped and not that anything with regards to running the cert monitor
	// actually work. Those are tested in the cert-monitor package.

	_, configDir, builderOpts := testSetupAutoConf(t)

	cfgFile := filepath.Join(configDir, "test.json")
	require.NoError(t, ioutil.WriteFile(cfgFile, []byte(`{
		"auto_config": {"enabled": true, "intro_token": "blarg", "server_addresses": ["198.18.0.1", "198.18.0.2:8398", "198.18.0.3:8399", "127.0.0.1:1234"]}, "verify_outgoing": true
	}`), 0600))

	builderOpts.ConfigFiles = append(builderOpts.ConfigFiles, cfgFile)
	directRPC := &mockDirectRPC{}
	certMon := &mockCertMonitor{}

	certMon.On("Start").Return((<-chan struct{})(nil), nil).Once()
	certMon.On("Stop").Return(true).Once()

	conf := new(Config).
		WithBuilderOpts(builderOpts).
		WithDirectRPC(directRPC).
		WithCertMonitor(certMon)

	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)
	cfg, err := ac.ReadConfig()
	require.NoError(t, err)
	ac.config = cfg

	require.NoError(t, ac.Start(context.Background()))
	require.True(t, ac.Stop())

	certMon.AssertExpectations(t)
	directRPC.AssertExpectations(t)
}

func TestFallBackTLS(t *testing.T) {
	_, configDir, builderOpts := testSetupAutoConf(t)

	cfgFile := filepath.Join(configDir, "test.json")
	require.NoError(t, ioutil.WriteFile(cfgFile, []byte(`{
		"auto_config": {"enabled": true, "intro_token": "blarg", "server_addresses": ["127.0.0.1:8300"]}, "verify_outgoing": true
	}`), 0600))

	builderOpts.ConfigFiles = append(builderOpts.ConfigFiles, cfgFile)

	directRPC := mockDirectRPC{}

	populateResponse := func(val interface{}) {
		resp, ok := val.(*pbautoconf.AutoConfigResponse)
		require.True(t, ok)
		resp.Config = &pbconfig.Config{
			PrimaryDatacenter: "primary",
			TLS: &pbconfig.TLS{
				VerifyServerHostname: true,
			},
		}

		resp.CARoots = &pbconnect.CARoots{
			ActiveRootID: "active",
			TrustDomain:  "trust",
			Roots: []*pbconnect.CARoot{
				{
					ID:           "active",
					Name:         "foo",
					SerialNumber: 42,
					SigningKeyID: "blarg",
					NotBefore:    &types.Timestamp{Seconds: 5000, Nanos: 100},
					NotAfter:     &types.Timestamp{Seconds: 10000, Nanos: 9009},
					RootCert:     "not an actual cert",
					Active:       true,
				},
			},
		}
		resp.Certificate = &pbconnect.IssuedCert{
			SerialNumber: "1234",
			CertPEM:      "not a cert",
			Agent:        "foo",
			AgentURI:     "spiffe://blarg/agent/client/dc/foo/id/foo",
			ValidAfter:   &types.Timestamp{Seconds: 6000},
			ValidBefore:  &types.Timestamp{Seconds: 7000},
		}
		resp.ExtraCACertificates = []string{"blarg"}
	}

	expectedRequest := pbautoconf.AutoConfigRequest{
		Datacenter: "dc1",
		Node:       "autoconf",
		JWT:        "blarg",
	}

	directRPC.On(
		"RPC",
		"dc1",
		"autoconf",
		&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8300},
		"AutoConfig.InitialConfiguration",
		&expectedRequest,
		&pbautoconf.AutoConfigResponse{}).Return(populateResponse)

	// setup the mock certificate monitor we don't expect it to be used
	// as the FallbackTLS method is mainly used by the certificate monitor
	// if for some reason it fails to renew the TLS certificate in time.
	certMon := mockCertMonitor{}

	conf := new(Config).
		WithBuilderOpts(builderOpts).
		WithDirectRPC(&directRPC).
		WithCertMonitor(&certMon)
	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)
	ac.config, err = ac.ReadConfig()
	require.NoError(t, err)

	actual, err := ac.FallbackTLS(context.Background())
	require.NoError(t, err)
	expected := &structs.SignedResponse{
		ConnectCARoots: structs.IndexedCARoots{
			ActiveRootID: "active",
			TrustDomain:  "trust",
			Roots: []*structs.CARoot{
				{
					ID:           "active",
					Name:         "foo",
					SerialNumber: 42,
					SigningKeyID: "blarg",
					NotBefore:    time.Unix(5000, 100),
					NotAfter:     time.Unix(10000, 9009),
					RootCert:     "not an actual cert",
					Active:       true,
				},
			},
		},
		IssuedCert: structs.IssuedCert{
			SerialNumber: "1234",
			CertPEM:      "not a cert",
			Agent:        "foo",
			AgentURI:     "spiffe://blarg/agent/client/dc/foo/id/foo",
			ValidAfter:   time.Unix(6000, 0),
			ValidBefore:  time.Unix(7000, 0),
		},
		ManualCARoots:        []string{"blarg"},
		VerifyServerHostname: true,
	}
	// have to just verify that the private key was put in here but we then
	// must zero it out so that the remaining equality check will pass
	require.NotEmpty(t, actual.IssuedCert.PrivateKeyPEM)
	actual.IssuedCert.PrivateKeyPEM = ""
	require.Equal(t, expected, actual)

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
	certMon.AssertExpectations(t)
}
