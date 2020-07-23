package autoconf

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto/pbautoconf"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-discover"
	discoverk8s "github.com/hashicorp/go-discover/provider/k8s"
	"github.com/hashicorp/go-hclog"
)

const (
	// autoConfigFileName is the name of the file that the agent auto-config settings are
	// stored in within the data directory
	autoConfigFileName = "auto-config.json"
)

// DirectRPC is the interface that needs to be satisifed for AutoConfig to be able to perform
// direct RPCs against individual servers. This should not use
type DirectRPC interface {
	RPC(dc string, node string, addr net.Addr, method string, args interface{}, reply interface{}) error
}

type options struct {
	logger          hclog.Logger
	directRPC       DirectRPC
	tlsConfigurator *tlsutil.Configurator
	builderOpts     config.BuilderOpts
	waiter          *lib.RetryWaiter
	overrides       []config.Source
}

// Option represents one point of configurability for the New function
// when creating a new AutoConfig object
type Option func(*options)

// WithLogger will cause the created AutoConfig type to use the provided logger
func WithLogger(logger hclog.Logger) Option {
	return func(opt *options) {
		opt.logger = logger
	}
}

// WithTLSConfigurator will cause the created AutoConfig type to use the provided configurator
func WithTLSConfigurator(tlsConfigurator *tlsutil.Configurator) Option {
	return func(opt *options) {
		opt.tlsConfigurator = tlsConfigurator
	}
}

// WithConnectionPool will cause the created AutoConfig type to use the provided connection pool
func WithDirectRPC(directRPC DirectRPC) Option {
	return func(opt *options) {
		opt.directRPC = directRPC
	}
}

// WithBuilderOpts will cause the created AutoConfig type to use the provided CLI builderOpts
func WithBuilderOpts(builderOpts config.BuilderOpts) Option {
	return func(opt *options) {
		opt.builderOpts = builderOpts
	}
}

// WithRetryWaiter will cause the created AutoConfig type to use the provided retry waiter
func WithRetryWaiter(waiter *lib.RetryWaiter) Option {
	return func(opt *options) {
		opt.waiter = waiter
	}
}

// WithOverrides is used to provide a config source to append to the tail sources
// during config building. It is really only useful for testing to tune non-user
// configurable tunables to make various tests converge more quickly than they
// could otherwise.
func WithOverrides(overrides ...config.Source) Option {
	return func(opt *options) {
		opt.overrides = overrides
	}
}

// AutoConfig is all the state necessary for being able to parse a configuration
// as well as perform the necessary RPCs to perform Agent Auto Configuration.
//
// NOTE: This struct and methods on it are not currently thread/goroutine safe.
// However it doesn't spawn any of its own go routines yet and is used in a
// synchronous fashion. In the future if either of those two conditions change
// then we will need to add some locking here. I am deferring that for now
// to help ease the review of this already large PR.
type AutoConfig struct {
	config          *config.RuntimeConfig
	builderOpts     config.BuilderOpts
	logger          hclog.Logger
	directRPC       DirectRPC
	tlsConfigurator *tlsutil.Configurator
	autoConfigData  string
	waiter          *lib.RetryWaiter
	overrides       []config.Source
}

func flattenOptions(opts []Option) options {
	var flat options
	for _, opt := range opts {
		opt(&flat)
	}
	return flat
}

// New creates a new AutoConfig object for providing automatic
// Consul configuration.
func New(options ...Option) (*AutoConfig, error) {
	flat := flattenOptions(options)

	if flat.directRPC == nil {
		return nil, fmt.Errorf("must provide a direct RPC delegate")
	}

	if flat.tlsConfigurator == nil {
		return nil, fmt.Errorf("must provide a TLS configurator")
	}

	logger := flat.logger
	if logger == nil {
		logger = hclog.NewNullLogger()
	} else {
		logger = logger.Named(logging.AutoConfig)
	}

	waiter := flat.waiter
	if waiter == nil {
		waiter = lib.NewRetryWaiter(1, 0, 10*time.Minute, lib.NewJitterRandomStagger(25))
	}

	ac := &AutoConfig{
		builderOpts:     flat.builderOpts,
		logger:          logger,
		directRPC:       flat.directRPC,
		tlsConfigurator: flat.tlsConfigurator,
		waiter:          waiter,
		overrides:       flat.overrides,
	}

	return ac, nil
}

// LoadConfig will build the configuration including the extraHead source injected
// after all other defaults but before any user supplied configuration and the overrides
// source injected as the final source in the configuration parsing chain.
func LoadConfig(builderOpts config.BuilderOpts, extraHead config.Source, overrides ...config.Source) (*config.RuntimeConfig, []string, error) {
	b, err := config.NewBuilder(builderOpts)
	if err != nil {
		return nil, nil, err
	}

	if extraHead.Data != "" {
		b.Head = append(b.Head, extraHead)
	}

	if len(overrides) != 0 {
		b.Tail = append(b.Tail, overrides...)
	}

	cfg, err := b.BuildAndValidate()
	if err != nil {
		return nil, nil, err
	}

	return &cfg, b.Warnings, nil
}

// ReadConfig will parse the current configuration and inject any
// auto-config sources if present into the correct place in the parsing chain.
func (ac *AutoConfig) ReadConfig() (*config.RuntimeConfig, error) {
	src := config.Source{
		Name:   autoConfigFileName,
		Format: "json",
		Data:   ac.autoConfigData,
	}

	cfg, warnings, err := LoadConfig(ac.builderOpts, src, ac.overrides...)
	if err != nil {
		return cfg, err
	}

	for _, w := range warnings {
		ac.logger.Warn(w)
	}

	ac.config = cfg
	return cfg, nil
}

// restorePersistedAutoConfig will attempt to load the persisted auto-config
// settings from the data directory. It returns true either when there was an
// unrecoverable error or when the configuration was successfully loaded from
// disk. Recoverable errors, such as "file not found" are suppressed and this
// method will return false for the first boolean.
func (ac *AutoConfig) restorePersistedAutoConfig() (bool, error) {
	if ac.config.DataDir == "" {
		// no data directory means we don't have anything to potentially load
		return false, nil
	}

	path := filepath.Join(ac.config.DataDir, autoConfigFileName)
	ac.logger.Debug("attempting to restore any persisted configuration", "path", path)

	content, err := ioutil.ReadFile(path)
	if err == nil {
		ac.logger.Info("restored persisted configuration", "path", path)
		ac.autoConfigData = string(content)
		return true, nil
	}

	if !os.IsNotExist(err) {
		return true, fmt.Errorf("failed to load %s: %w", path, err)
	}

	// ignore non-existence errors as that is an indicator that we haven't
	// performed the auto configuration before
	return false, nil
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
	if ac.config == nil {
		config, err := ac.ReadConfig()
		if err != nil {
			return nil, err
		}

		ac.config = config
	}

	if !ac.config.AutoConfig.Enabled {
		return ac.config, nil
	}

	ready, err := ac.restorePersistedAutoConfig()
	if err != nil {
		return nil, err
	}

	if !ready {
		ac.logger.Info("retrieving initial agent auto configuration remotely")
		if err := ac.getInitialConfiguration(ctx); err != nil {
			return nil, err
		}
	}

	// re-read the configuration now that we have our initial auto-config
	config, err := ac.ReadConfig()
	if err != nil {
		return nil, err
	}

	ac.config = config
	return ac.config, nil
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

// autoConfigHosts is responsible for taking the list of server addresses and
// resolving any go-discover provider invocations. It will then return a list
// of hosts. These might be hostnames and is expected that DNS resolution may
// be performed after this function runs. Additionally these may contain ports
// so SplitHostPort could also be necessary.
func (ac *AutoConfig) autoConfigHosts() ([]string, error) {
	servers := ac.config.AutoConfig.ServerAddresses

	providers := make(map[string]discover.Provider)
	for k, v := range discover.Providers {
		providers[k] = v
	}
	providers["k8s"] = &discoverk8s.Provider{}

	disco, err := discover.New(
		discover.WithUserAgent(lib.UserAgent()),
		discover.WithProviders(providers),
	)

	if err != nil {
		return nil, fmt.Errorf("Failed to create go-discover resolver: %w", err)
	}

	var addrs []string
	for _, addr := range servers {
		switch {
		case strings.Contains(addr, "provider="):
			resolved, err := disco.Addrs(addr, ac.logger.StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true}))
			if err != nil {
				ac.logger.Error("failed to resolve go-discover auto-config servers", "configuration", addr, "err", err)
				continue
			}

			addrs = append(addrs, resolved...)
			ac.logger.Debug("discovered auto-config servers", "servers", resolved)
		default:
			addrs = append(addrs, addr)
		}
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no auto-config server addresses available for use")
	}

	return addrs, nil
}

// resolveHost will take a single host string and convert it to a list of TCPAddrs
// This will process any port in the input as well as looking up the hostname using
// normal DNS resolution.
func (ac *AutoConfig) resolveHost(hostPort string) []net.TCPAddr {
	port := ac.config.ServerPort
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			host = hostPort
		} else {
			ac.logger.Warn("error splitting host address into IP and port", "address", hostPort, "error", err)
			return nil
		}
	} else {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			ac.logger.Warn("Parsed port is not an integer", "port", portStr, "error", err)
			return nil
		}
	}

	// resolve the host to a list of IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		ac.logger.Warn("IP resolution failed", "host", host, "error", err)
		return nil
	}

	var addrs []net.TCPAddr
	for _, ip := range ips {
		addrs = append(addrs, net.TCPAddr{IP: ip, Port: port})
	}

	return addrs
}

// recordAutoConfigReply takes an AutoConfig RPC reply records it with the agent
// This will persist the configuration to disk (unless in dev mode running without
// a data dir) and will reload the configuration.
func (ac *AutoConfig) recordAutoConfigReply(reply *pbautoconf.AutoConfigResponse) error {
	// overwrite the auto encrypt DNS SANs with the ones specified in the auto_config stanza
	if len(ac.config.AutoConfig.DNSSANs) > 0 && reply.Config.AutoEncrypt != nil {
		reply.Config.AutoEncrypt.DNSSAN = ac.config.AutoConfig.DNSSANs
	}

	// overwrite the auto encrypt IP SANs with the ones specified in the auto_config stanza
	if len(ac.config.AutoConfig.IPSANs) > 0 && reply.Config.AutoEncrypt != nil {
		var ips []string
		for _, ip := range ac.config.AutoConfig.IPSANs {
			ips = append(ips, ip.String())
		}
		reply.Config.AutoEncrypt.IPSAN = ips
	}

	conf, err := json.Marshal(translateConfig(reply.Config))
	if err != nil {
		return fmt.Errorf("failed to encode auto-config configuration as JSON: %w", err)
	}

	ac.autoConfigData = string(conf)

	if ac.config.DataDir == "" {
		ac.logger.Debug("not persisting auto-config settings because there is no data directory")
		return nil
	}

	path := filepath.Join(ac.config.DataDir, autoConfigFileName)

	err = ioutil.WriteFile(path, conf, 0660)
	if err != nil {
		return fmt.Errorf("failed to write auto-config configurations: %w", err)
	}

	ac.logger.Debug("auto-config settings were persisted to disk")

	return nil
}

// getInitialConfigurationOnce will perform full server to TCPAddr resolution and
// loop through each host trying to make the AutoConfig.InitialConfiguration RPC call. When
// successful the bool return will be true and the err value will indicate whether we
// successfully recorded the auto config settings (persisted to disk and stored internally
// on the AutoConfig object)
func (ac *AutoConfig) getInitialConfigurationOnce(ctx context.Context) (bool, error) {
	token, err := ac.introToken()
	if err != nil {
		return false, err
	}

	request := pbautoconf.AutoConfigRequest{
		Datacenter: ac.config.Datacenter,
		Node:       ac.config.NodeName,
		Segment:    ac.config.SegmentName,
		JWT:        token,
	}

	var reply pbautoconf.AutoConfigResponse

	servers, err := ac.autoConfigHosts()
	if err != nil {
		return false, err
	}

	for _, s := range servers {
		// try each IP to see if we can successfully make the request
		for _, addr := range ac.resolveHost(s) {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}

			ac.logger.Debug("making AutoConfig.InitialConfiguration RPC", "addr", addr.String())
			if err = ac.directRPC.RPC(ac.config.Datacenter, ac.config.NodeName, &addr, "AutoConfig.InitialConfiguration", &request, &reply); err != nil {
				ac.logger.Error("AutoConfig.InitialConfiguration RPC failed", "addr", addr.String(), "error", err)
				continue
			}

			return true, ac.recordAutoConfigReply(&reply)
		}
	}

	return false, ctx.Err()
}

// getInitialConfiguration implements a loop to retry calls to getInitialConfigurationOnce.
// It uses the RetryWaiter on the AutoConfig object to control how often to attempt
// the initial configuration process. It is also canceallable by cancelling the provided context.
func (ac *AutoConfig) getInitialConfiguration(ctx context.Context) error {
	// this resets the failures so that we will perform immediate request
	wait := ac.waiter.Success()
	for {
		select {
		case <-wait:
			done, err := ac.getInitialConfigurationOnce(ctx)
			if done {
				return err
			}
			if err != nil {
				ac.logger.Error(err.Error())
			}
			wait = ac.waiter.Failed()
		case <-ctx.Done():
			ac.logger.Info("interrupted during initial auto configuration", "err", ctx.Err())
			return ctx.Err()
		}
	}
}
