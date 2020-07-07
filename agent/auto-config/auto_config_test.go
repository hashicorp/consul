package autoconf

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbautoconf"
	"github.com/hashicorp/consul/proto/pbconfig"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockDirectRPC struct {
	mock.Mock
}

func (m *mockDirectRPC) RPC(dc string, node string, addr net.Addr, method string, args interface{}, reply interface{}) error {
	retValues := m.Called(dc, node, addr, method, args, reply)
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

func TestNew(t *testing.T) {
	type testCase struct {
		opts     []Option
		err      string
		validate func(t *testing.T, ac *AutoConfig)
	}

	cases := map[string]testCase{
		"no-direct-rpc": {
			opts: []Option{
				WithTLSConfigurator(&tlsutil.Configurator{}),
			},
			err: "must provide a direct RPC delegate",
		},
		"no-tls-configurator": {
			opts: []Option{
				WithDirectRPC(&mockDirectRPC{}),
			},
			err: "must provide a TLS configurator",
		},
		"ok": {
			opts: []Option{
				WithTLSConfigurator(&tlsutil.Configurator{}),
				WithDirectRPC(&mockDirectRPC{}),
			},
			validate: func(t *testing.T, ac *AutoConfig) {
				t.Helper()
				require.NotNil(t, ac.logger)
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ac, err := New(tcase.opts...)
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
	ac, err := New(WithBuilderOpts(builderOpts), WithTLSConfigurator(&tlsutil.Configurator{}), WithDirectRPC(&directRPC))
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
	ac, err := New(WithBuilderOpts(builderOpts), WithTLSConfigurator(&tlsutil.Configurator{}), WithDirectRPC(&directRPC))
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
	response := &pbconfig.Config{
		PrimaryDatacenter: "primary",
	}
	data, err := json.Marshal(translateConfig(response))
	require.NoError(t, err)
	require.NoError(t, ioutil.WriteFile(persistedFile, data, 0600))

	directRPC := mockDirectRPC{}

	ac, err := New(WithBuilderOpts(builderOpts), WithTLSConfigurator(&tlsutil.Configurator{}), WithDirectRPC(&directRPC))
	require.NoError(t, err)
	require.NotNil(t, ac)

	cfg, err := ac.InitialConfiguration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "primary", cfg.PrimaryDatacenter)

	// ensure no RPC was made
	directRPC.AssertExpectations(t)
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
		}
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

	ac, err := New(WithBuilderOpts(builderOpts), WithTLSConfigurator(&tlsutil.Configurator{}), WithDirectRPC(&directRPC))
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
	ac, err := New(WithBuilderOpts(builderOpts), WithTLSConfigurator(&tlsutil.Configurator{}), WithDirectRPC(&directRPC), WithRetryWaiter(waiter))
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
