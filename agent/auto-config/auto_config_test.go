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
			config: Config{
				Loader: func(source config.Source) (cfg *config.RuntimeConfig, warnings []string, err error) {
					return nil, nil, nil
				},
			},
			err: "must provide a direct RPC delegate",
		},

		"no-config-loader": {
			err: "must provide a config loader",
		},
		"ok": {
			config: Config{
				DirectRPC: &mockDirectRPC{},
				Loader: func(source config.Source) (cfg *config.RuntimeConfig, warnings []string, err error) {
					return nil, nil, nil
				},
			},
			validate: func(t *testing.T, ac *AutoConfig) {
				t.Helper()
				require.NotNil(t, ac.logger)
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ac, err := New(tcase.config)
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
			Loader: func(source config.Source) (*config.RuntimeConfig, []string, error) {
				cfg, _, err := source.Parse()
				if err != nil {
					return nil, nil, err
				}
				return &config.RuntimeConfig{
					DevMode:  true,
					NodeName: *cfg.NodeName,
				}, nil, nil
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

func setupRuntimeConfig(t *testing.T) *config.RuntimeConfig {
	t.Helper()

	dataDir := testutil.TempDir(t, "auto-config")
	t.Cleanup(func() { os.RemoveAll(dataDir) })

	rtConfig := &config.RuntimeConfig{
		DataDir:    dataDir,
		Datacenter: "dc1",
		NodeName:   "autoconf",
		BindAddr:   &net.IPAddr{IP: net.ParseIP("127.0.0.1")},
	}
	return rtConfig
}

func TestInitialConfiguration_disabled(t *testing.T) {
	rtConfig := setupRuntimeConfig(t)

	directRPC := new(mockDirectRPC)
	directRPC.Test(t)
	conf := Config{
		DirectRPC: directRPC,
		Loader: func(source config.Source) (*config.RuntimeConfig, []string, error) {
			rtConfig.PrimaryDatacenter = "primary"
			rtConfig.AutoConfig.Enabled = false
			return rtConfig, nil, nil
		},
	}
	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)
	require.NoFileExists(t, filepath.Join(rtConfig.DataDir, autoConfigFileName))

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
}

func TestInitialConfiguration_cancelled(t *testing.T) {
	rtConfig := setupRuntimeConfig(t)

	directRPC := new(mockDirectRPC)
	directRPC.Test(t)
	expectedRequest := pbautoconf.AutoConfigRequest{
		Datacenter: "dc1",
		Node:       "autoconf",
		JWT:        "blarg",
	}

	directRPC.On("RPC", "dc1", "autoconf", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8300}, "AutoConfig.InitialConfiguration", &expectedRequest, mock.Anything).Return(fmt.Errorf("injected error")).Times(0)
	conf := Config{
		DirectRPC: directRPC,
		Loader: func(source config.Source) (*config.RuntimeConfig, []string, error) {
			rtConfig.PrimaryDatacenter = "primary"
			rtConfig.AutoConfig = config.AutoConfig{
				Enabled:         true,
				IntroToken:      "blarg",
				ServerAddresses: []string{"127.0.0.1:8300"},
			}
			rtConfig.VerifyOutgoing = true
			return rtConfig, nil, nil
		},
	}
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
	rtConfig := setupRuntimeConfig(t)

	// persist an auto config response to the data dir where it is expected
	persistedFile := filepath.Join(rtConfig.DataDir, autoConfigFileName)
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

	directRPC := new(mockDirectRPC)
	directRPC.Test(t)

	// setup the mock certificate monitor to ensure that the initial state gets
	// updated appropriately during config restoration.
	certMon := new(mockCertMonitor)
	certMon.Test(t)
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

	conf := Config{
		DirectRPC: directRPC,
		Loader: func(source config.Source) (*config.RuntimeConfig, []string, error) {
			if err := setPrimaryDatacenterFromSource(rtConfig, source); err != nil {
				return nil, nil, err
			}
			rtConfig.AutoConfig = config.AutoConfig{
				Enabled:         true,
				IntroToken:      "blarg",
				ServerAddresses: []string{"127.0.0.1:8300"},
			}
			rtConfig.VerifyOutgoing = true
			return rtConfig, nil, nil
		},
		CertMonitor: certMon,
	}
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

func setPrimaryDatacenterFromSource(rtConfig *config.RuntimeConfig, source config.Source) error {
	if source != nil {
		cfg, _, err := source.Parse()
		if err != nil {
			return err
		}
		rtConfig.PrimaryDatacenter = *cfg.PrimaryDatacenter
	}
	return nil
}

func TestInitialConfiguration_success(t *testing.T) {
	rtConfig := setupRuntimeConfig(t)

	directRPC := new(mockDirectRPC)
	directRPC.Test(t)

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
	certMon := new(mockCertMonitor)
	certMon.Test(t)
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

	conf := Config{
		DirectRPC: directRPC,
		Loader: func(source config.Source) (*config.RuntimeConfig, []string, error) {
			if err := setPrimaryDatacenterFromSource(rtConfig, source); err != nil {
				return nil, nil, err
			}
			rtConfig.AutoConfig = config.AutoConfig{
				Enabled:         true,
				IntroToken:      "blarg",
				ServerAddresses: []string{"127.0.0.1:8300"},
			}
			rtConfig.VerifyOutgoing = true
			return rtConfig, nil, nil
		},
		CertMonitor: certMon,
	}
	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)

	// the file was written to.
	persistedFile := filepath.Join(rtConfig.DataDir, autoConfigFileName)
	require.FileExists(t, persistedFile)

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
	certMon.AssertExpectations(t)
}

func TestInitialConfiguration_retries(t *testing.T) {
	rtConfig := setupRuntimeConfig(t)

	directRPC := new(mockDirectRPC)
	directRPC.Test(t)

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

	conf := Config{
		DirectRPC: directRPC,
		Loader: func(source config.Source) (*config.RuntimeConfig, []string, error) {
			if err := setPrimaryDatacenterFromSource(rtConfig, source); err != nil {
				return nil, nil, err
			}
			rtConfig.AutoConfig = config.AutoConfig{
				Enabled:    true,
				IntroToken: "blarg",
				ServerAddresses: []string{
					"198.18.0.1:8300",
					"198.18.0.2:8398",
					"198.18.0.3:8399",
					"127.0.0.1:1234",
				},
			}
			rtConfig.VerifyOutgoing = true
			return rtConfig, nil, nil
		},
		Waiter: lib.NewRetryWaiter(2, 0, 1*time.Millisecond, nil),
	}
	ac, err := New(conf)
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)

	// the file was written to.
	persistedFile := filepath.Join(rtConfig.DataDir, autoConfigFileName)
	require.FileExists(t, persistedFile)

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
}

func TestAutoConfig_StartStop(t *testing.T) {
	// currently the only thing running for autoconf is just the cert monitor
	// so this test only needs to ensure that the cert monitor is started and
	// stopped and not that anything with regards to running the cert monitor
	// actually work. Those are tested in the cert-monitor package.

	rtConfig := setupRuntimeConfig(t)

	directRPC := &mockDirectRPC{}
	directRPC.Test(t)
	certMon := &mockCertMonitor{}
	certMon.Test(t)

	certMon.On("Start").Return((<-chan struct{})(nil), nil).Once()
	certMon.On("Stop").Return(true).Once()

	conf := Config{
		DirectRPC: directRPC,
		Loader: func(source config.Source) (*config.RuntimeConfig, []string, error) {
			rtConfig.AutoConfig = config.AutoConfig{
				Enabled:    true,
				IntroToken: "blarg",
				ServerAddresses: []string{
					"198.18.0.1",
					"198.18.0.2:8398",
					"198.18.0.3:8399",
					"127.0.0.1:1234",
				},
			}
			rtConfig.VerifyOutgoing = true
			return rtConfig, nil, nil
		},
		CertMonitor: certMon,
	}
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
	rtConfig := setupRuntimeConfig(t)

	directRPC := new(mockDirectRPC)
	directRPC.Test(t)

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
	certMon := new(mockCertMonitor)

	conf := Config{
		DirectRPC: directRPC,
		Loader: func(source config.Source) (*config.RuntimeConfig, []string, error) {
			rtConfig.AutoConfig = config.AutoConfig{
				Enabled:         true,
				IntroToken:      "blarg",
				ServerAddresses: []string{"127.0.0.1:8300"},
			}
			rtConfig.VerifyOutgoing = true
			return rtConfig, nil, nil
		},
		CertMonitor: certMon,
	}
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
