package agent

import (
	"crypto/x509"
	"fmt"
	"time"

	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/tlsutil"
)

var CertExpirationGauges = []prometheus.GaugeDefinition{
	{
		Name: metricsKeyAgentTLSCertExpiry,
		Help: "Seconds until the agent tls certificate expires. Updated every hour",
	},
}

var metricsKeyAgentTLSCertExpiry = []string{"agent", "tls", "cert", "expiry"}

// tlsCertExpirationMonitor returns a CertExpirationMonitor which will
// monitor the expiration of the certificate used for agent TLS.
func tlsCertExpirationMonitor(c *tlsutil.Configurator, logger hclog.Logger) consul.CertExpirationMonitor {
	return consul.CertExpirationMonitor{
		Key:    metricsKeyAgentTLSCertExpiry,
		Logger: logger,
		Query: func() (time.Duration, error) {
			raw := c.Cert()
			if raw == nil {
				return 0, fmt.Errorf("tls not enabled")
			}

			cert, err := x509.ParseCertificate(raw.Certificate[0])
			if err != nil {
				return 0, fmt.Errorf("failed to parse agent tls cert: %w", err)
			}
			return time.Until(cert.NotAfter), nil
		},
	}
}
