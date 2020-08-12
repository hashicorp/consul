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

	// CertMonitor is the Connect TLS Certificate Monitor to be used for ongoing
	// certificate renewals and connect CA roots updates. This field is not
	// strictly required but if not provided the TLS certificates retrieved
	// through by the AutoConfig.InitialConfiguration RPC will not be used
	// or renewed.
	CertMonitor CertMonitor

	// Loader merges source with the existing FileSources and returns the complete
	// RuntimeConfig.
	Loader func(source config.Source) (cfg *config.RuntimeConfig, warnings []string, err error)
}
