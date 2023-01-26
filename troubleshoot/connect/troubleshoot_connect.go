package troubleshoot

import (
	"fmt"
	"net"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"

	"github.com/hashicorp/consul/api"
)

type Troubleshoot struct {
	client         *api.Client
	envoyAddr      net.IPAddr
	envoyAdminPort string

	TroubleshootInfo
}

type TroubleshootInfo struct {
	envoyClusters   *envoy_admin_v3.Clusters
	envoyConfigDump *envoy_admin_v3.ConfigDump
	envoyCerts      *envoy_admin_v3.Certificates
	envoyStats      EnvoyStats
}

type EnvoyStats []envoy_admin_v3.SimpleMetric

func NewTroubleshoot(envoyIP *net.IPAddr, envoyPort string) (*Troubleshoot, error) {
	cfg := api.DefaultConfig()
	c, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Troubleshoot{
		client:         c,
		envoyAddr:      *envoyIP,
		envoyAdminPort: envoyPort,
	}, nil
}

func (t *Troubleshoot) RunAllTests(upstreamSNI string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (t *Troubleshoot) GetUpstreams() ([]string, error) {

	return nil, fmt.Errorf("not implemented")
}
