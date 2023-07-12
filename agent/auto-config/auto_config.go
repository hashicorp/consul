package autoconf

import (
	"context"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto/pbautoconf"
)

// AutoConfig is all the state necessary for being able to parse a configuration
// as well as perform the necessary RPCs to perform Agent Auto Configuration.
type AutoConfig struct {
	sync.Mutex

	acConfig           Config
	logger             hclog.Logger
	cache              Cache
	waiter             *retry.Waiter
	config             *config.RuntimeConfig
	autoConfigResponse *pbautoconf.AutoConfigResponse
	autoConfigSource   config.Source

	running bool
	done    chan struct{}
	// cancel is used to cancel the entire AutoConfig
	// go routine. This is the main field protected
	// by the mutex as it being non-nil indicates that
	// the go routine has been started and is stoppable.
	// note that it doesn't indcate that the go routine
	// is currently running.
	cancel context.CancelFunc

	// cancelWatches is used to cancel the existing
	// cache watches regarding the agents certificate. This is
	// mainly only necessary when the Agent token changes.
	cancelWatches context.CancelFunc

	// cacheUpdates is the chan used to have the cache
	// send us back events
	cacheUpdates chan cache.UpdateEvent

	// tokenUpdates is the struct used to receive
	// events from the token store when the Agent
	// token is updated.
	tokenUpdates token.Notifier
}

// New creates a new AutoConfig object for providing automatic Consul configuration.
func New(config Config) (*AutoConfig, error) {
	switch {
	case config.Loader == nil:
		return nil, fmt.Errorf("must provide a config loader")
	case config.DirectRPC == nil:
		return nil, fmt.Errorf("must provide a direct RPC delegate")
	case config.Cache == nil:
		return nil, fmt.Errorf("must provide a cache")
	case config.TLSConfigurator == nil:
		return nil, fmt.Errorf("must provide a TLS configurator")
	case config.Tokens == nil:
		return nil, fmt.Errorf("must provide a token store")
	}

	if config.FallbackLeeway == 0 {
		config.FallbackLeeway = 10 * time.Second
	}
	if config.FallbackRetry == 0 {
		config.FallbackRetry = time.Minute
	}

	logger := config.Logger
	if logger == nil {
		logger = hclog.NewNullLogger()
	} else {
		logger = logger.Named(logging.AutoConfig)
	}

	if config.Waiter == nil {
		config.Waiter = &retry.Waiter{
			MinFailures: 1,
			MaxWait:     10 * time.Minute,
			Jitter:      retry.NewJitter(25),
		}
	}

	if err := config.EnterpriseConfig.validateAndFinalize(); err != nil {
		return nil, err
	}

	return &AutoConfig{
		acConfig: config,
		logger:   logger,
	}, nil
}

// ReadConfig will parse the current configuration and inject any
// auto-config sources if present into the correct place in the parsing chain.
func (ac *AutoConfig) ReadConfig() (*config.RuntimeConfig, error) {
	ac.Lock()
	defer ac.Unlock()
	result, err := ac.acConfig.Loader(ac.autoConfigSource)
	if err != nil {
		return result.RuntimeConfig, err
	}

	for _, w := range result.Warnings {
		ac.logger.Warn(w)
	}

	ac.config = result.RuntimeConfig
	return ac.config, nil
}

// InitialConfiguration will perform a one-time RPC request to the configured servers
// to retrieve various cluster wide configurations. See the proto/pbautoconf/auto_config.proto
// file for a complete reference of what configurations can be applied in this manner.
// The returned configuration will be the new configuration with any auto-config settings
// already applied. If AutoConfig is not enabled this method will just parse any
// local configuration and return the built runtime configuration.
//
// The context passed in can be used to cancel the retrieval of the initial configuration
// like when receiving a signal during startup.
func (ac *AutoConfig) InitialConfiguration(ctx context.Context) (*config.RuntimeConfig, error) {
	if err := ac.maybeLoadConfig(); err != nil {
		return nil, err
	}

	switch {
	case ac.config.AutoConfig.Enabled:
		resp, err := ac.readPersistedAutoConfig()
		if err != nil {
			return nil, err
		}

		if resp == nil {
			ac.logger.Info("retrieving initial agent auto configuration remotely")
			resp, err = ac.getInitialConfiguration(ctx)
			if err != nil {
				return nil, err
			}
		}

		ac.logger.Debug("updating auto-config settings")
		if err = ac.recordInitialConfiguration(resp); err != nil {
			return nil, err
		}

		// re-read the configuration now that we have our initial auto-config
		config, err := ac.ReadConfig()
		if err != nil {
			return nil, err
		}

		ac.config = config
		return ac.config, nil
	case ac.config.AutoEncryptTLS:
		certs, err := ac.autoEncryptInitialCerts(ctx)
		if err != nil {
			return nil, err
		}

		if err := ac.setInitialTLSCertificates(certs); err != nil {
			return nil, err
		}

		ac.logger.Info("automatically upgraded to TLS")
		return ac.config, nil
	default:
		return ac.config, nil
	}
}

// maybeLoadConfig will read the Consul configuration using the
// provided config loader if and only if the config field of
// the struct is nil. When it does this it will fill in that
// field. If the config field already is non-nil then this
// is a noop.
func (ac *AutoConfig) maybeLoadConfig() error {
	if ac.config == nil {
		config, err := ac.ReadConfig()
		if err != nil {
			return err
		}

		ac.config = config
	}
	return nil
}

// introToken is responsible for determining the correct intro token to use
// when making the initial AutoConfig.InitialConfiguration RPC request.
func (ac *AutoConfig) introToken() (string, error) {
	conf := ac.config.AutoConfig
	// without an intro token or intro token file we cannot do anything
	if conf.IntroToken == "" && conf.IntroTokenFile == "" {
		return "", fmt.Errorf("neither intro_token or intro_token_file settings are not configured")
	}

	token := conf.IntroToken
	if token == "" {
		// load the intro token from the file
		content, err := ioutil.ReadFile(conf.IntroTokenFile)
		if err != nil {
			return "", fmt.Errorf("Failed to read intro token from file: %w", err)
		}

		token = string(content)

		if token == "" {
			return "", fmt.Errorf("intro_token_file did not contain any token")
		}
	}

	return token, nil
}

// recordInitialConfiguration is responsible for recording the AutoConfigResponse from
// the AutoConfig.InitialConfiguration RPC. It is an all-in-one function to do the following
//   - update the Agent token in the token store
func (ac *AutoConfig) recordInitialConfiguration(resp *pbautoconf.AutoConfigResponse) error {
	ac.autoConfigResponse = resp

	ac.autoConfigSource = config.LiteralSource{
		Name:   autoConfigFileName,
		Config: translateConfig(resp.Config),
	}

	// we need to re-read the configuration to determine what the correct ACL
	// token to push into the token store is. Any user provided token will override
	// any AutoConfig generated token.
	config, err := ac.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to fully resolve configuration: %w", err)
	}

	// ignoring the return value which would indicate a change in the token
	_ = ac.acConfig.Tokens.UpdateAgentToken(config.ACLTokens.ACLAgentToken, token.TokenSourceConfig)

	// extra a structs.SignedResponse from the AutoConfigResponse for use in cache prepopulation
	signed, err := extractSignedResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to extract certificates from the auto-config response: %w", err)
	}

	// prepopulate the cache
	if err = ac.populateCertificateCache(signed); err != nil {
		return fmt.Errorf("failed to populate the cache with certificate responses: %w", err)
	}

	// update the TLS configurator with the latest certificates
	if err := ac.updateTLSFromResponse(resp); err != nil {
		return err
	}

	return ac.persistAutoConfig(resp)
}

// getInitialConfigurationOnce will perform full server to TCPAddr resolution and
// loop through each host trying to make the AutoConfig.InitialConfiguration RPC call. When
// successful the bool return will be true and the err value will indicate whether we
// successfully recorded the auto config settings (persisted to disk and stored internally
// on the AutoConfig object)
func (ac *AutoConfig) getInitialConfigurationOnce(ctx context.Context, csr string, key string) (*pbautoconf.AutoConfigResponse, error) {
	token, err := ac.introToken()
	if err != nil {
		return nil, err
	}

	request := pbautoconf.AutoConfigRequest{
		Datacenter: ac.config.Datacenter,
		Node:       ac.config.NodeName,
		Segment:    ac.config.SegmentName,
		Partition:  ac.config.PartitionOrEmpty(),
		JWT:        token,
		CSR:        csr,
	}

	var resp pbautoconf.AutoConfigResponse

	servers, err := ac.autoConfigHosts()
	if err != nil {
		return nil, err
	}

	for _, s := range servers {
		// try each IP to see if we can successfully make the request
		for _, addr := range ac.resolveHost(s) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			ac.logger.Debug("making AutoConfig.InitialConfiguration RPC", "addr", addr.String())
			if err = ac.acConfig.DirectRPC.RPC(ac.config.Datacenter, ac.config.NodeName, &addr, "AutoConfig.InitialConfiguration", &request, &resp); err != nil {
				ac.logger.Error("AutoConfig.InitialConfiguration RPC failed", "addr", addr.String(), "error", err)
				continue
			}
			ac.logger.Debug("AutoConfig.InitialConfiguration RPC was successful")

			// update the Certificate with the private key we generated locally
			if resp.Certificate != nil {
				resp.Certificate.PrivateKeyPEM = key
			}

			return &resp, nil
		}
	}

	return nil, fmt.Errorf("No server successfully responded to the auto-config request")
}

// getInitialConfiguration implements a loop to retry calls to getInitialConfigurationOnce.
// It uses the RetryWaiter on the AutoConfig object to control how often to attempt
// the initial configuration process. It is also canceallable by cancelling the provided context.
func (ac *AutoConfig) getInitialConfiguration(ctx context.Context) (*pbautoconf.AutoConfigResponse, error) {
	// generate a CSR
	csr, key, err := ac.generateCSR()
	if err != nil {
		return nil, err
	}

	ac.acConfig.Waiter.Reset()
	for {
		resp, err := ac.getInitialConfigurationOnce(ctx, csr, key)
		switch {
		case err == nil && resp != nil:
			return resp, nil
		case err != nil:
			ac.logger.Error(err.Error())
		default:
			ac.logger.Error("No error returned when fetching configuration from the servers but no response was either")
		}

		if err := ac.acConfig.Waiter.Wait(ctx); err != nil {
			ac.logger.Info("interrupted during initial auto configuration", "err", err)
			return nil, err
		}
	}
}

func (ac *AutoConfig) Start(ctx context.Context) error {
	ac.Lock()
	defer ac.Unlock()

	if !ac.config.AutoConfig.Enabled && !ac.config.AutoEncryptTLS {
		return nil
	}

	if ac.running || ac.cancel != nil {
		return fmt.Errorf("AutoConfig is already running")
	}

	// create the top level context to control the go
	// routine executing the `run` method
	ctx, cancel := context.WithCancel(ctx)

	// create the channel to get cache update events through
	// really we should only ever get 10 updates
	ac.cacheUpdates = make(chan cache.UpdateEvent, 10)

	// setup the cache watches
	cancelCertWatches, err := ac.setupCertificateCacheWatches(ctx)
	if err != nil {
		cancel()
		return fmt.Errorf("error setting up cache watches: %w", err)
	}

	// start the token update notifier
	ac.tokenUpdates = ac.acConfig.Tokens.Notify(token.TokenKindAgent)

	// store the cancel funcs
	ac.cancel = cancel
	ac.cancelWatches = cancelCertWatches

	ac.running = true
	ac.done = make(chan struct{})
	go ac.run(ctx, ac.done)

	ac.logger.Info("auto-config started")
	return nil
}

func (ac *AutoConfig) Done() <-chan struct{} {
	ac.Lock()
	defer ac.Unlock()

	if ac.done != nil {
		return ac.done
	}

	// return a closed channel to indicate that we are already done
	done := make(chan struct{})
	close(done)
	return done
}

func (ac *AutoConfig) IsRunning() bool {
	ac.Lock()
	defer ac.Unlock()
	return ac.running
}

func (ac *AutoConfig) Stop() bool {
	ac.Lock()
	defer ac.Unlock()

	if !ac.running {
		return false
	}

	if ac.cancel != nil {
		ac.cancel()
	}

	return true
}
