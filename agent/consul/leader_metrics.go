package consul

import (
	"context"
	"fmt"
	"github.com/hashicorp/consul/agent/connect"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/logging"
)

var metricsKeyMeshRootCAExpiry = []string{"mesh", "active-root-ca", "expiry"}
var metricsKeyMeshIntermediateCAExpiry = []string{"mesh", "active-intermediate-ca", "expiry"}

var CertExpirationGauges = []prometheus.GaugeDefinition{
	{
		Name: metricsKeyMeshRootCAExpiry,
		Help: "Seconds until the service mesh root certificate expires.",
	},
	{
		Name: metricsKeyMeshIntermediateCAExpiry,
		Help: "Seconds until the service mesh intermediate certificate expires.",
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

func intermediateCAExpiryMonitor(s *Server) certExpirationMonitor {
	return certExpirationMonitor{
		Key: metricsKeyMeshIntermediateCAExpiry,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: s.config.Datacenter},
		},
		Logger: s.logger.Named(logging.Connect),
		Query: func() (time.Duration, error) {
			provider, _ := s.caManager.getCAProvider()
			certPem, err := provider.ActiveIntermediate()
			if err != nil {
				return time.Duration(0), err
			}
			cert, err := connect.ParseCert(certPem)
			if err != nil {
				return time.Duration(0), err
			}
			return time.Until(cert.NotAfter), nil
		},
	}
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
