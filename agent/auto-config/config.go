// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package autoconf

import (
	"context"
	"crypto/x509"
	"net"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/lib/retry"
)

// DirectRPC is the interface that needs to be satisifed for AutoConfig to be able to perform
// direct RPCs against individual servers. This will not be used for any ongoing RPCs as once
// the agent gets configured, it can go through the normal RPC means of selecting a available
// server automatically.
type DirectRPC interface {
	RPC(dc string, node string, addr net.Addr, method string, args interface{}, reply interface{}) error
}

// Cache is an interface to represent the methods of the
// agent/cache.Cache struct that we care about
type Cache interface {
	Notify(ctx context.Context, t string, r cache.Request, correlationID string, ch chan<- cache.UpdateEvent) error
	Prepopulate(t string, result cache.FetchResult, dc string, peerName string, token string, key string) error
}

// LeafCertManager is an interface to represent the methods of the
// agent/leafcert.Manager struct that we care about
type LeafCertManager interface {
	Prepopulate(
		ctx context.Context,
		key string,
		index uint64,
		value *structs.IssuedCert,
		authorityKeyID string,
	) error
	Notify(ctx context.Context, req *leafcert.ConnectCALeafRequest, correlationID string, ch chan<- cache.UpdateEvent) error
}

// ServerProvider is an interface that can be used to find one server in the local DC known to
// the agent via Gossip
type ServerProvider interface {
	FindLANServer() *metadata.Server
}

// TLSConfigurator is an interface of the methods on the tlsutil.Configurator that we will require at
// runtime.
type TLSConfigurator interface {
	UpdateAutoTLS(manualCAPEMs, connectCAPEMs []string, pub, priv string, verifyServerHostname bool) error
	UpdateAutoTLSCA([]string) error
	UpdateAutoTLSCert(pub, priv string) error
	AutoEncryptCert() *x509.Certificate
}

// TokenStore is an interface of the methods we will need to use from the token.Store.
type TokenStore interface {
	AgentToken() string
	UpdateAgentToken(secret string, source token.TokenSource) bool
	Notify(kind token.TokenKind) token.Notifier
	StopNotify(notifier token.Notifier)
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

	// ServerProvider is the interfaced to be used by AutoConfig to find any
	// known servers during fallback operations.
	ServerProvider ServerProvider

	// Waiter is used during retrieval of the initial configuration.
	// When around of requests fails we will
	// wait and eventually make another round of requests (1 round
	// is trying the RPC once against each configured server addr). The
	// waiting implements some backoff to prevent from retrying these RPCs
	// too often. This field is not required and if left unset a waiter will
	// be used that has a max wait duration of 10 minutes and a randomized
	// jitter of 25% of the wait time. Setting this is mainly useful for
	// testing purposes to allow testing out the retrying functionality without
	// having the test take minutes/hours to complete.
	Waiter *retry.Waiter

	// Loader merges source with the existing FileSources and returns the complete
	// RuntimeConfig.
	Loader func(source config.Source) (config.LoadResult, error)

	// TLSConfigurator is the shared TLS Configurator. AutoConfig will update the
	// auto encrypt/auto config certs as they are renewed.
	TLSConfigurator TLSConfigurator

	// Cache is an object implementing our Cache interface. The Cache
	// used at runtime must be able to handle Roots watches
	Cache Cache

	// LeafCertManager is an object implementing our LeafCertManager interface.
	LeafCertManager LeafCertManager

	// FallbackLeeway is the amount of time after certificate expiration before
	// invoking the fallback routine. If not set this will default to 10s.
	FallbackLeeway time.Duration

	// FallbackRetry is the duration between Fallback invocations when the configured
	// fallback routine returns an error. If not set this will default to 1m.
	FallbackRetry time.Duration

	// Tokens is the shared token store. It is used to retrieve the current
	// agent token as well as getting notifications when that token is updated.
	// This field is required.
	Tokens TokenStore

	// EnterpriseConfig is the embedded specific enterprise configurations
	EnterpriseConfig
}
