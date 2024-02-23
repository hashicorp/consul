// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"fmt"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/dnsutil"
	"net"

	"github.com/miekg/dns"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/logging"
)

// DNSRouter is a mock for Router that can be used for testing.
//
//go:generate mockery --name DNSRouter --inpackage
type DNSRouter interface {
	HandleRequest(req *dns.Msg, reqCtx Context, remoteAddress net.Addr) *dns.Msg
	ServeDNS(w dns.ResponseWriter, req *dns.Msg)
	GetConfig() *RouterDynamicConfig
	ReloadConfig(newCfg *config.RuntimeConfig) error
}

// Server is used to expose service discovery queries using a DNS interface.
// It implements the agent.dnsServer interface.
type Server struct {
	*dns.Server           // Used for setting up listeners
	Router      DNSRouter // Used to routes and parse DNS requests

	logger hclog.Logger
}

// Config represent all the DNS configuration required to construct a DNS server.
type Config struct {
	AgentConfig                 *config.RuntimeConfig
	EntMeta                     acl.EnterpriseMeta
	Logger                      hclog.Logger
	Processor                   DiscoveryQueryProcessor
	TokenFunc                   func() string
	TranslateAddressFunc        func(dc string, addr string, taggedAddresses map[string]string, accept dnsutil.TranslateAddressAccept) string
	TranslateServiceAddressFunc func(dc string, address string, taggedAddresses map[string]structs.ServiceAddress, accept dnsutil.TranslateAddressAccept) string
}

// NewServer creates a new DNS server.
func NewServer(config Config) (*Server, error) {
	router, err := NewRouter(config)
	if err != nil {
		return nil, fmt.Errorf("error creating DNS router: %w", err)
	}

	srv := &Server{
		Router: router,
		logger: config.Logger.Named(logging.DNS),
	}
	return srv, nil
}

// ListenAndServe starts the DNS server.
func (d *Server) ListenAndServe(network, addr string, notif func()) error {
	d.Server = &dns.Server{
		Addr:              addr,
		Net:               network,
		Handler:           d.Router,
		NotifyStartedFunc: notif,
	}
	if network == "udp" {
		d.UDPSize = 65535
	}
	return d.Server.ListenAndServe()
}

// ReloadConfig hot-reloads the server config with new parameters under config.RuntimeConfig.DNS*
func (d *Server) ReloadConfig(newCfg *config.RuntimeConfig) error {
	return d.Router.ReloadConfig(newCfg)
}

// Shutdown stops the DNS server.
func (d *Server) Shutdown() {
	if d.Server != nil {
		d.logger.Info("Stopping server",
			"protocol", "DNS",
			"address", d.Server.Addr,
			"network", d.Server.Net,
		)
		err := d.Server.Shutdown()
		if err != nil {
			d.logger.Error("Error stopping DNS server", "error", err)
		}
	}
	d.Router = nil
}

// GetAddr is a function to return the server address if is not nil.
func (d *Server) GetAddr() string {
	if d.Server != nil {
		return d.Server.Addr
	}
	return ""
}
