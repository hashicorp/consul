// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scada

import (
	"fmt"
	"net"

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/go-hclog"
	libscada "github.com/hashicorp/hcp-scada-provider"
	"github.com/hashicorp/hcp-scada-provider/capability"
	cloud "github.com/hashicorp/hcp-sdk-go/clients/cloud-shared/v1/models"
	hcpcfg "github.com/hashicorp/hcp-sdk-go/config"
)

// Provider is the interface used in the rest of Consul core when using SCADA, it is aliased here to the same interface
// provided by the hcp-scada-provider library. If the interfaces needs to be extended in the future it can be done so
// with minimal impact on the rest of the codebase.
//
//go:generate mockery --name Provider --with-expecter --inpackage
type Provider interface {
	libscada.SCADAProvider
	UpdateHCPConfig(cfg config.CloudConfig) error
}

const (
	scadaConsulServiceKey = "consul"
)

type scadaProvider struct {
	libscada.SCADAProvider
	logger hclog.Logger
}

// New returns an initialized SCADA provider with a zero configuration.
// It can listen but cannot start until UpdateHCPConfig is called with
// a configuration that provides credentials to contact HCP.
func New(logger hclog.Logger) (*scadaProvider, error) {
	// Create placeholder resource link
	resourceLink := cloud.HashicorpCloudLocationLink{
		Type:     "no-op",
		ID:       "no-op",
		Location: &cloud.HashicorpCloudLocationLocation{},
	}

	// Configure with an empty HCP configuration
	hcpConfig, err := hcpcfg.NewHCPConfig(hcpcfg.WithoutBrowserLogin())
	if err != nil {
		return nil, fmt.Errorf("failed to configure SCADA provider: %w", err)
	}

	pvd, err := libscada.New(&libscada.Config{
		Service:   scadaConsulServiceKey,
		HCPConfig: hcpConfig,
		Resource:  resourceLink,
		Logger:    logger,
	})
	if err != nil {
		return nil, err
	}

	return &scadaProvider{pvd, logger}, nil
}

// UpdateHCPConfig updates the SCADA provider with the given HCP
// configurations.
func (p *scadaProvider) UpdateHCPConfig(cfg config.CloudConfig) error {
	resource, err := cfg.Resource()
	if err != nil {
		return err
	}

	hcpCfg, err := cfg.HCPConfig()
	if err != nil {
		return err
	}

	err = p.UpdateConfig(&libscada.Config{
		Service:   scadaConsulServiceKey,
		HCPConfig: hcpCfg,
		Resource:  *resource.Link(),
		Logger:    p.logger,
	})
	if err != nil {
		return err
	}

	return nil
}

// IsCapability takes a net.Addr and returns true if it is a SCADA capability.Addr
func IsCapability(a net.Addr) bool {
	_, ok := a.(*capability.Addr)
	return ok
}
