// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"crypto/x509"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/tlsutil"
)

var metricsKeyAgentTLSCertExpiry = []string{"agent", "tls", "cert", "expiry"}

var testCertExpirationMonitorInterval time.Duration

func tlsCertRole(isServer bool) string {
	if isServer {
		return "server"
	}
	return "client"
}

func certExpirationGauges(datacenter, partition, nodeName, role string) []prometheus.GaugeDefinition {
	return []prometheus.GaugeDefinition{
		{
			Name: metricsKeyAgentTLSCertExpiry,
			Help: "Seconds until the agent tls certificate expires. Updated every hour",
			ConstLabels: []metrics.Label{
				{Name: "datacenter", Value: datacenter},
				{Name: "partition", Value: acl.PartitionOrDefault(partition)},
				{Name: "node", Value: nodeName},
				{Name: "role", Value: role},
			},
		},
	}
}

// tlsCertExpirationMonitor returns a CertExpirationMonitor which will
// monitor the expiration of the certificate used for agent TLS.
func tlsCertExpirationMonitor(c *tlsutil.Configurator, datacenter, partition, nodeName, role string, criticalDays int, warningDays int, logger hclog.Logger) consul.CertExpirationMonitor {
	labels := []metrics.Label{
		{Name: "datacenter", Value: datacenter},
		{Name: "partition", Value: acl.PartitionOrDefault(partition)},
		{Name: "node", Value: nodeName},
		{Name: "role", Value: role},
	}
	return consul.CertExpirationMonitor{
		Key:                   metricsKeyAgentTLSCertExpiry,
		Labels:                labels,
		Logger:                logger,
		CriticalThresholdDays: criticalDays,
		WarningThresholdDays:  warningDays,
		Interval:              testCertExpirationMonitorInterval,
		Query: func() (time.Duration, time.Duration, error) {
			raw := c.Cert()
			if raw == nil {
				return 0, 0, fmt.Errorf("tls not enabled")
			}

			cert, err := x509.ParseCertificate(raw.Certificate[0])
			if err != nil {
				return 0, 0, fmt.Errorf("failed to parse agent tls cert: %w", err)
			}

			lifetime := time.Since(cert.NotBefore) + time.Until(cert.NotAfter)
			return lifetime, time.Until(cert.NotAfter), nil
		},
	}
}
