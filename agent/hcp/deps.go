package hcp

import (
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/consul/agent/hcp/scada"
	"github.com/hashicorp/go-hclog"
)

// Deps contains the interfaces that the rest of Consul core depends on for HCP integration.
type Deps struct {
	Client   Client
	Provider scada.Provider
}

func NewDeps(cfg config.CloudConfig, logger hclog.Logger) (d Deps, err error) {
	d.Client, err = NewClient(cfg)
	if err != nil {
		return
	}

	d.Provider, err = scada.New(cfg, logger.Named("hcp.scada"))
	return
}
