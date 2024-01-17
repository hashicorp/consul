// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/armon/go-radix"
	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/discovery"
	"github.com/hashicorp/consul/agent/structs"
)

// TODO (v2-dns): metrics

// RouterDynamicConfig is the dynamic configuration that can be hot-reloaded
type RouterDynamicConfig struct {
	ARecordLimit          int
	DisableCompression    bool
	EnableDefaultFailover bool // TODO (v2-dns): plumbing required for this new V2 setting. This is the agent configured default
	EnableTruncate        bool
	NodeMetaTXT           bool
	NodeTTL               time.Duration
	Recursors             []string
	RecursorTimeout       time.Duration
	RecursorStrategy      structs.RecursorStrategy
	SOAConfig             SOAConfig
	// TTLRadix sets service TTLs by prefix, eg: "database-*"
	TTLRadix *radix.Tree
	// TTLStrict sets TTLs to service by full name match. It Has higher priority than TTLRadix
	TTLStrict      map[string]time.Duration
	UDPAnswerLimit int
}

type SOAConfig struct {
	Refresh uint32 // 3600 by default
	Retry   uint32 // 600
	Expire  uint32 // 86400
	Minttl  uint32 // 0
}

// DiscoveryQueryProcessor is an interface that can be used by any consumer requesting Service Discovery results.
// This could be attached to a gRPC endpoint in the future in addition to DNS.
// Making this an interface means testing the router with a mock is trivial.
type DiscoveryQueryProcessor interface {
	QueryByName(*discovery.Query, discovery.Context) ([]*discovery.Result, error)
	QueryByIP(net.IP, discovery.Context) ([]*discovery.Result, error)
}

// Router replaces miekg/dns.ServeMux with a simpler router that only checks for the 2-3 valid domains
// that Consul supports and forwards to a single DiscoveryQueryProcessor handler. If there is no match, it will recurse.
type Router struct {
	processor DiscoveryQueryProcessor
	domain    string
	altDomain string
	logger    hclog.Logger

	tokenFunc func() string

	defaultNamespace string
	defaultPartition string

	// TODO (v2-dns): default locality for request context?

	// dynamicConfig stores the config as an atomic value (for hot-reloading).
	// It is always of type *RouterDynamicConfig
	dynamicConfig atomic.Value
}

var _ = dns.Handler(&Router{})

func NewRouter(cfg Config) (*Router, error) {
	router := &Router{
		// TODO (v2-dns): implement stub
	}

	if err := router.ReloadConfig(cfg.AgentConfig); err != nil {
		return nil, err
	}
	return router, nil
}

// HandleRequest is used to process and individual DNS request. It returns a message in success or fail cases.
func (r Router) HandleRequest(req *dns.Msg, reqCtx discovery.Context, remoteAddress net.Addr) *dns.Msg {
	cfg := r.dynamicConfig.Load().(*RouterDynamicConfig)

	// TODO (v2-dns): implement HandleRequest. This is just temporary
	return createServerFailureResponse(req, cfg, false)

	// Parse fields of the message

	// Route the request to the appropriate destination
	// 1. r.processor.QueryByName
	// 2. r.processor.QueryByIP
	// 3. recurse

	// Serialize the output

}

// ServeDNS implements the miekg/dns.Handler interface
func (r Router) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	reqCtx := r.defaultAgentDNSRequestContext()
	out := r.HandleRequest(req, reqCtx, w.RemoteAddr())
	w.WriteMsg(out)
}

// GetDynamicRouterConfig takes global config and creates the config used by DNS server
func GetDynamicRouterConfig(conf *config.RuntimeConfig) (*RouterDynamicConfig, error) {
	cfg := &RouterDynamicConfig{
		// TODO (v2-dns)
	}

	return cfg, nil
}

// ReloadConfig hot-reloads the router config with new parameters
func (r Router) ReloadConfig(newCfg *config.RuntimeConfig) error {
	cfg, err := GetDynamicRouterConfig(newCfg)
	if err != nil {
		return fmt.Errorf("error loading DNS config: %w", err)
	}
	r.dynamicConfig.Store(cfg)
	return nil
}

func (r Router) defaultAgentDNSRequestContext() discovery.Context {
	return discovery.Context{
		// TODO (v2-dns): implement stub
	}
}

func createServerFailureResponse(req *dns.Msg, cfg *RouterDynamicConfig, recursionAvailable bool) *dns.Msg {
	// Return a SERVFAIL message
	m := &dns.Msg{}
	m.SetReply(req)
	m.Compress = !cfg.DisableCompression
	m.SetRcode(req, dns.RcodeServerFailure)
	m.RecursionAvailable = recursionAvailable
	return m
}
