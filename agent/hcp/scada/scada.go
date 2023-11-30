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
	"github.com/hashicorp/hcp-sdk-go/resource"
)

// Provider is the interface used in the rest of Consul core when using SCADA, it is aliased here to the same interface
// provided by the hcp-scada-provider library. If the interfaces needs to be extended in the future it can be done so
// with minimal impact on the rest of the codebase.
//
//go:generate mockery --name Provider --with-expecter --inpackage
type Provider interface {
	libscada.SCADAProvider
}

const (
	scadaConsulServiceKey = "consul"
)

func New(cfg config.CloudConfig, logger hclog.Logger) (Provider, error) {
	resource, err := resource.FromString(cfg.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cloud resource_id: %w", err)
	}

	hcpConfig, err := cfg.HCPConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build HCPConfig: %w", err)
	}

	pvd, err := libscada.New(&libscada.Config{
		Service:   scadaConsulServiceKey,
		HCPConfig: hcpConfig,
		Resource:  *resource.Link(),
		Logger:    logger,
	})
	if err != nil {
		return nil, err
	}

	return pvd, nil
}

// IsCapability takes a net.Addr and returns true if it is a SCADA capability.Addr
func IsCapability(a net.Addr) bool {
	_, ok := a.(*capability.Addr)
	return ok
}
