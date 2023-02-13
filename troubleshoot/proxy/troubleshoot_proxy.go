package troubleshoot

import (
	"fmt"
	"net"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/troubleshoot/validate"
)

const (
	listeners       string = "type.googleapis.com/envoy.admin.v3.ListenersConfigDump"
	clusters        string = "type.googleapis.com/envoy.admin.v3.ClustersConfigDump"
	routes          string = "type.googleapis.com/envoy.admin.v3.RoutesConfigDump"
	endpoints       string = "type.googleapis.com/envoy.admin.v3.EndpointsConfigDump"
	bootstrap       string = "type.googleapis.com/envoy.admin.v3.BootstrapConfigDump"
	httpConnManager string = "type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager"
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

func (t *Troubleshoot) RunAllTests(upstreamEnvoyID, upstreamIP string) (validate.Messages, error) {
	var allTestMessages validate.Messages

	// Get all info from proxy to set up validations.
	err := t.GetEnvoyConfigDump()
	if err != nil {
		return nil, fmt.Errorf("unable to get Envoy config dump: cannot connect to Envoy: %w", err)
	}
	err = t.getEnvoyClusters()
	if err != nil {
		return nil, fmt.Errorf("unable to get Envoy clusters: cannot connect to Envoy: %w", err)
	}
	certs, err := t.getEnvoyCerts()
	if err != nil {
		return nil, fmt.Errorf("unable to get Envoy certificates: cannot connect to Envoy: %w", err)
	}
	indexedResources, err := ProxyConfigDumpToIndexedResources(t.envoyConfigDump)
	if err != nil {
		return nil, fmt.Errorf("unable to index Envoy resources: %w", err)
	}

	// Validate certs.
	messages := t.validateCerts(certs)
	allTestMessages = append(allTestMessages, messages...)
	if errors := messages.Errors(); len(errors) == 0 {
		msg := validate.Message{
			Success: true,
			Message: "certificates are valid",
		}
		allTestMessages = append(allTestMessages, msg)
	}

	// getStats usage example
	messages, err = t.troubleshootStats()
	if err != nil {
		return nil, fmt.Errorf("unable to get stats: %w", err)
	}
	allTestMessages = append(allTestMessages, messages...)

	// Validate listeners, routes, clusters, endpoints.
	messages = Validate(indexedResources, upstreamEnvoyID, upstreamIP, true, t.envoyClusters)
	allTestMessages = append(allTestMessages, messages...)
	if errors := messages.Errors(); len(errors) == 0 {
		msg := validate.Message{
			Success: true,
			Message: "upstream resources are valid",
		}
		allTestMessages = append(allTestMessages, msg)
	}

	return allTestMessages, nil
}
