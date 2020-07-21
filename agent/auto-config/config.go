package autoconf

import (
	"context"
	"net"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-hclog"
)

// DirectRPC is the interface that needs to be satisifed for AutoConfig to be able to perform
// direct RPCs against individual servers. This will not be used for any ongoing RPCs as once
// the agent gets configured, it can go through the normal RPC means of selecting a available
// server automatically.
type DirectRPC interface {
	RPC(dc string, node string, addr net.Addr, method string, args interface{}, reply interface{}) error
}

// CertMonitor is the interface that needs to be satisfied for AutoConfig to be able to
// setup monitoring of the Connect TLS certificate after we first get it.
type CertMonitor interface {
	Update(*structs.SignedResponse) error
	Start(context.Context) (<-chan struct{}, error)
	Stop() bool
}

// Config contains all the tunables for AutoConfig
type Config struct {
	// Logger is any logger that should be utilized. If not provided,
	// then no logs will be emitted.
	Logger hclog.Logger

	// DirectRPC is the interface to be used by AutoConfig to make the
	// AutoConfig.InitialConfiguration RPCs for generating the bootstrap
	// configuration. Setting this field is required.
	DirectRPC DirectRPC

	// BuilderOpts are any configuration building options that should be
	// used when loading the Consul configuration. This is mostly a pass
	// through from what the CLI will generate. While this option is
	// not strictly required, not setting it will prevent AutoConfig
	// from doing anything useful. Enabling AutoConfig requires a
	// CLI flag or a config file (also specified by the CLI) flag.
	// So without providing the opts its equivalent to using the
	// configuration of not specifying anything to the consul agent
	// cli.
	BuilderOpts config.BuilderOpts

	// Waiter is a RetryWaiter to be used during retrieval of the
	// initial configuration. When a round of requests fails we will
	// wait and eventually make another round of requests (1 round
	// is trying the RPC once against each configured server addr). The
	// waiting implements some backoff to prevent from retrying these RPCs
	// to often. This field is not required and if left unset a waiter will
	// be used that has a max wait duration of 10 minutes and a randomized
	// jitter of 25% of the wait time. Setting this is mainly useful for
	// testing purposes to allow testing out the retrying functionality without
	// having the test take minutes/hours to complete.
	Waiter *lib.RetryWaiter

	// Overrides are a list of configuration sources to append to the tail of
	// the config builder. This field is optional and mainly useful for testing
	// to override values that would be otherwise not user-settable.
	Overrides []config.Source

	// CertMonitor is the Connect TLS Certificate Monitor to be used for ongoing
	// certificate renewals and connect CA roots updates. This field is not
	// strictly required but if not provided the TLS certificates retrieved
	// through by the AutoConfig.InitialConfiguration RPC will not be used
	// or renewed.
	CertMonitor CertMonitor
}

// WithLogger will cause the created AutoConfig type to use the provided logger
func (c *Config) WithLogger(logger hclog.Logger) *Config {
	c.Logger = logger
	return c
}

// WithConnectionPool will cause the created AutoConfig type to use the provided connection pool
func (c *Config) WithDirectRPC(directRPC DirectRPC) *Config {
	c.DirectRPC = directRPC
	return c
}

// WithBuilderOpts will cause the created AutoConfig type to use the provided CLI builderOpts
func (c *Config) WithBuilderOpts(builderOpts config.BuilderOpts) *Config {
	c.BuilderOpts = builderOpts
	return c
}

// WithRetryWaiter will cause the created AutoConfig type to use the provided retry waiter
func (c *Config) WithRetryWaiter(waiter *lib.RetryWaiter) *Config {
	c.Waiter = waiter
	return c
}

// WithOverrides is used to provide a config source to append to the tail sources
// during config building. It is really only useful for testing to tune non-user
// configurable tunables to make various tests converge more quickly than they
// could otherwise.
func (c *Config) WithOverrides(overrides ...config.Source) *Config {
	c.Overrides = overrides
	return c
}

// WithCertMonitor is used to provide a certificate monitor to the auto-config.
// This monitor is responsible for renewing the agents TLS certificate and keeping
// the connect CA roots up to date.
func (c *Config) WithCertMonitor(certMonitor CertMonitor) *Config {
	c.CertMonitor = certMonitor
	return c
}
