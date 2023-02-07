package troubleshoot

import (
	"fmt"
	"net"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"
)

const (
	listeners string = "type.googleapis.com/envoy.admin.v3.ListenersConfigDump"
	clusters  string = "type.googleapis.com/envoy.admin.v3.ClustersConfigDump"
	routes    string = "type.googleapis.com/envoy.admin.v3.RoutesConfigDump"
	endpoints string = "type.googleapis.com/envoy.admin.v3.EndpointsConfigDump"
	bootstrap string = "type.googleapis.com/envoy.admin.v3.BootstrapConfigDump"
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
	envoyStats      []*envoy_admin_v3.SimpleMetric
}

func NewTroubleshoot(envoyIP *net.IPAddr, envoyPort string) (*Troubleshoot, error) {
	cfg := api.DefaultConfig()
	c, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	if envoyIP == nil {
		return nil, fmt.Errorf("envoy address is empty")
	}

	return &Troubleshoot{
		client:         c,
		envoyAddr:      *envoyIP,
		envoyAdminPort: envoyPort,
	}, nil
}

func (t *Troubleshoot) RunAllTests(envoyID string) ([]string, error) {
	var resultErr error
	var output []string

	// Validate certs
	certs, err := t.getEnvoyCerts()
	if err != nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("unable to get certs: %w", err))
	}

	if certs != nil && len(certs.GetCertificates()) != 0 {
		err = t.validateCerts(certs)
		if err != nil {
			resultErr = multierror.Append(resultErr, fmt.Errorf("unable to validate certs: %w", err))
		} else {
			output = append(output, "certs are valid")
		}

	} else {
		resultErr = multierror.Append(resultErr, fmt.Errorf("no certificate found"))

	}

	// getStats usage example
	// rejectionStats, err := t.getEnvoyStats("update_rejected")
	// if err != nil {
	// 	resultErr = multierror.Append(resultErr, err)
	// }

	// Validate listeners, routes, clusters, endpoints
	t.GetEnvoyConfigDump()
	t.getEnvoyClusters()

	indexedResources, err := ProxyConfigDumpToIndexedResources(t.envoyConfigDump)
	if err != nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("unable to index resources: %v", err))
	}

	err = Validate(indexedResources, envoyID, "", true, t.envoyClusters)
	if err != nil {
		resultErr = multierror.Append(resultErr, fmt.Errorf("unable to validate proxy config: %v", err))
	}
	return output, resultErr
}

func (t *Troubleshoot) GetUpstreams() ([]string, error) {

	upstreams := []string{}

	err := t.GetEnvoyConfigDump()
	if err != nil {
		return nil, err
	}

	for _, cfg := range t.envoyConfigDump.Configs {
		switch cfg.TypeUrl {
		case listeners:
			lcd := &envoy_admin_v3.ListenersConfigDump{}

			err := proto.Unmarshal(cfg.GetValue(), lcd)
			if err != nil {
				return nil, err
			}

			for _, listener := range lcd.GetDynamicListeners() {
				upstream := envoyID(listener.Name)
				if upstream != "" && upstream != "public_listener" &&
					upstream != "outbound_listener" &&
					upstream != "inbound_listener" {
					upstreams = append(upstreams, upstream)
				}
			}
		}
	}
	return upstreams, nil
}
