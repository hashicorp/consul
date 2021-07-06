package consul

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/consul/agent/connect/ca"

	"github.com/hashicorp/consul/agent/connect"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
)

var metricsKeyMeshRootCAExpiry = []string{"mesh", "active-root-ca", "expiry"}
var metricsKeyMeshPrimaryCAExpiry = []string{"mesh", "active-primary-dc-ca", "expiry"}
var metricsKeyMeshSecondaryCAExpiry = []string{"mesh", "active-secondary-dc-ca", "expiry"}

var CertExpirationGauges = []prometheus.GaugeDefinition{
	{
		Name: metricsKeyMeshRootCAExpiry,
		Help: "Seconds until the service mesh root certificate expires.",
	},
	{
		Name: metricsKeyMeshPrimaryCAExpiry,
		Help: "Seconds until the service mesh primary DC certificate expires.",
	},
	{
		Name: metricsKeyMeshSecondaryCAExpiry,
		Help: "Seconds until the service mesh secondary DC certificate expires.",
	},
}

func rootCAExpiryMonitor(s *Server) certExpirationMonitor {
	return certExpirationMonitor{
		Key: metricsKeyMeshRootCAExpiry,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: s.config.Datacenter},
		},
		Logger: s.logger.Named(logging.Connect),
		Query: func() (time.Duration, error) {
			state := s.fsm.State()
			_, root, err := state.CARootActive(nil)
			switch {
			case err != nil:
				return 0, fmt.Errorf("failed to retrieve root CA: %w", err)
			case root == nil:
				return 0, fmt.Errorf("no active root CA")
			}

			return time.Until(root.NotAfter), nil
		},
	}
}

func primaryCAExpiryMonitor(s *Server) certExpirationMonitor {
	return certExpirationMonitor{
		Key: metricsKeyMeshPrimaryCAExpiry,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: s.config.Datacenter},
		},
		Logger: s.logger.Named(logging.Connect),
		Query: func() (time.Duration, error) {

			isPrimary := s.config.Datacenter == s.config.PrimaryDatacenter
			if isPrimary {
				provider, _ := s.caManager.getCAProvider()

				if _, ok := provider.(ca.PrimaryUsesIntermediate); !ok {
					cert, err := getActiveIntermediate(s)
					if err != nil {
						return 0, err
					}
					return time.Until(cert.NotAfter), nil
				}

				state := s.fsm.State()
				_, root, err := state.CARootActive(nil)
				switch {
				case err != nil:
					return 0, fmt.Errorf("failed to retrieve root CA: %w", err)
				case root == nil:
					return 0, fmt.Errorf("no active root CA")
				}

				return time.Until(root.NotAfter), nil
			}
			return 0, nil
		},
	}
}

func secondaryCAExpiryMonitor(s *Server) certExpirationMonitor {
	return certExpirationMonitor{
		Key: metricsKeyMeshSecondaryCAExpiry,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: s.config.Datacenter},
		},
		Logger: s.logger.Named(logging.Connect),
		Query: func() (time.Duration, error) {
			isPrimary := s.config.Datacenter == s.config.PrimaryDatacenter
			if !isPrimary {
				cert, err := getActiveIntermediate(s)
				if err != nil {
					return 0, err
				}
				return time.Until(cert.NotAfter), nil
			}
			return 0, nil
		},
	}
}

func getActiveIntermediate(s *Server) (*x509.Certificate, error) {
	state := s.fsm.State()
	_, root, err := state.CARootActive(nil)
	if err != nil {
		return nil, err
	}

	// the CA used in a secondary DC is the active intermediate,
	// which is the last in the IntermediateCerts stack
	if len(root.IntermediateCerts) == 0 {
		return nil, errors.New("no intermediate available")
	}
	cert, err := connect.ParseCert(root.IntermediateCerts[len(root.IntermediateCerts)-1])
	if err != nil {
		return nil, err
	}
	return cert, nil
}

type certExpirationMonitor struct {
	Key    []string
	Labels []metrics.Label
	Logger hclog.Logger
	// Query is called at each interval. It should return the duration until the
	// certificate expires, or an error if the query failed.
	Query func() (time.Duration, error)
}

const certExpirationMonitorInterval = time.Hour

func (m certExpirationMonitor) monitor(ctx context.Context) error {
	ticker := time.NewTicker(certExpirationMonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			d, err := m.Query()
			if err != nil {
				m.Logger.Warn("failed to emit certificate expiry metric", "error", err)
			}
			expiry := d / time.Second
			metrics.SetGaugeWithLabels(m.Key, float32(expiry), m.Labels)
		}
	}
}
