package troubleshoot

import (
	"fmt"
	"net"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"github.com/pkg/errors"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"
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
	return &Troubleshoot{
		client:         c,
		envoyAddr:      *envoyIP,
		envoyAdminPort: envoyPort,
	}, nil
}

func (t *Troubleshoot) RunAllTests(upstreamEnvoyID string) ([]string, error) {
	var resultErr error
	var output []string

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
		resultErr = multierror.Append(resultErr, errors.New("no certificate found"))

	}

	// getStats usage example
	// rejectionStats, err := t.getEnvoyStats("update_rejected")
	// if err != nil {
	// 	resultErr = multierror.Append(resultErr, err)
	// }

	return output, resultErr
}

func (t *Troubleshoot) GetUpstreams() ([]string, error) {

	return nil, fmt.Errorf("not implemented")
}
