// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/logging"
)

var (
	metricsKeyMeshRootCAExpiry          = []string{"mesh", "active-root-ca", "expiry"}
	metricsKeyMeshActiveSigningCAExpiry = []string{"mesh", "active-signing-ca", "expiry"}
)

var LeaderCertExpirationGauges = []prometheus.GaugeDefinition{
	{
		Name: metricsKeyMeshRootCAExpiry,
		Help: "Seconds until the service mesh root certificate expires. Updated every hour",
	},
	{
		Name: metricsKeyMeshActiveSigningCAExpiry,
		Help: "Seconds until the service mesh signing certificate expires. Updated every hour",
	},
}

func rootCAExpiryMonitor(s *Server) CertExpirationMonitor {
	return CertExpirationMonitor{
		Key:    metricsKeyMeshRootCAExpiry,
		Logger: s.logger.Named(logging.Connect),
		Query: func() (time.Duration, time.Duration, error) {
			return getRootCAExpiry(s)
		},
		Server: s,
	}
}

func getRootCAExpiry(s *Server) (time.Duration, time.Duration, error) {
	state := s.fsm.State()
	_, root, err := state.CARootActive(nil)
	switch {
	case err != nil:
		return 0, 0, fmt.Errorf("failed to retrieve root CA: %w", err)
	case root == nil:
		return 0, 0, fmt.Errorf("no active root CA")
	}

	lifetime := time.Since(root.NotBefore) + time.Until(root.NotAfter)
	return lifetime, time.Until(root.NotAfter), nil
}

func signingCAExpiryMonitor(s *Server) CertExpirationMonitor {
	return CertExpirationMonitor{
		Key:    metricsKeyMeshActiveSigningCAExpiry,
		Logger: s.logger.Named(logging.Connect),
		Query: func() (time.Duration, time.Duration, error) {
			if s.caManager.isIntermediateUsedToSignLeaf() {
				return getActiveIntermediateExpiry(s)
			}
			return getRootCAExpiry(s)
		},
		Server: s,
	}
}

func getActiveIntermediateExpiry(s *Server) (time.Duration, time.Duration, error) {
	state := s.fsm.State()
	_, root, err := state.CARootActive(nil)
	switch {
	case err != nil:
		return 0, 0, fmt.Errorf("failed to retrieve root CA: %w", err)
	case root == nil:
		return 0, 0, fmt.Errorf("no active root CA")
	}

	// the CA used in a secondary DC is the active intermediate,
	// which is the last in the IntermediateCerts stack
	if len(root.IntermediateCerts) == 0 {
		return 0, 0, errors.New("no intermediate available")
	}
	cert, err := connect.ParseCert(root.IntermediateCerts[len(root.IntermediateCerts)-1])
	if err != nil {
		return 0, 0, err
	}

	lifetime := time.Since(cert.NotBefore) + time.Until(cert.NotAfter)
	return lifetime, time.Until(cert.NotAfter), nil
}

type CertExpirationMonitor struct {
	Key []string
	// Labels to be emitted along with the metric. It is very important that these
	// labels be included in the pre-declaration as well. Otherwise, if
	// telemetry.prometheus_retention_time is less than certExpirationMonitorInterval
	// then the metrics will expire before they are emitted again.
	Labels []metrics.Label
	Logger hclog.Logger
	// Query is called at each interval. It should return 2 durations, the full
	// lifespan of the certificate (NotBefore -> NotAfter) and the duration
	// until the certificate expires (Now -> NotAfter), or an error if the
	// query failed.
	Query  func() (time.Duration, time.Duration, error)
	Server *Server
}

const certExpirationMonitorInterval = time.Hour

func (m CertExpirationMonitor) Monitor(ctx context.Context) error {
	// Check if certificate telemetry is enabled
	if !m.Server.config.CertificateTelemetryEnabled {
		return nil
	}

	ticker := time.NewTicker(certExpirationMonitorInterval)
	defer ticker.Stop()

	logger := m.Logger.With("metric", strings.Join(m.Key, "."))

	emitMetric := func() {
		_, untilAfter, err := m.Query()
		if err != nil {
			logger.Warn("failed to emit certificate expiry metric", "error", err)
			return
		}

		daysRemaining := int(untilAfter.Hours() / 24)
		criticalDays := m.Server.config.CertificateTelemetryCriticalThresholdDays
		warningDays := m.Server.config.CertificateTelemetryWarningThresholdDays

		// Determine cert type and suggested action for logging
		key := strings.Join(m.Key, ":")
		var certType string
		var suggestedAction string
		var nodeName string

		switch key {
		case "mesh:active-root-ca:expiry":
			certType = "Root"
			suggestedAction = "manually rotate the root certificate"
		case "mesh:active-signing-ca:expiry":
			certType = "Intermediate"
			suggestedAction = "check consul logs for rotation issues"
		case "agent:tls:cert:expiry":
			certType = "Agent"
			// Try to extract node name from labels if available
			for _, label := range m.Labels {
				if label.Name == "node" {
					nodeName = label.Value
					break
				}
			}
			if nodeName != "" {
				suggestedAction = fmt.Sprintf("manually rotate this agent's certificate on node %s", nodeName)
			} else {
				suggestedAction = "manually rotate this agent's certificate"
			}
		}

		// Build log fields
		logFields := []interface{}{
			"cert_type", certType,
			"days_remaining", daysRemaining,
			"time_to_expiry", untilAfter,
			"expiration", time.Now().Add(untilAfter),
			"suggested_action", suggestedAction,
		}
		if nodeName != "" {
			logFields = append(logFields, "node", nodeName)
		}

		// Log based on threshold severity with detailed context
		if daysRemaining < criticalDays {
			logger.Error("certificate expiring soon", logFields...)
		} else if daysRemaining < warningDays {
			logger.Warn("certificate expiring soon", logFields...)
		}

		expiry := untilAfter / time.Second
		metrics.SetGaugeWithLabels(m.Key, float32(expiry), m.Labels)
	}

	// emit the metric immediately so that if a cert was just updated the
	// new metric will be updated to the new expiration time.
	emitMetric()

	for {
		select {
		case <-ctx.Done():
			// "Zero-out" the metric on exit so that when prometheus scrapes this
			// metric from a non-leader, it does not get a stale value.
			metrics.SetGaugeWithLabels(m.Key, float32(math.NaN()), m.Labels)
			return nil
		case <-ticker.C:
			emitMetric()
		}
	}
}

// initLeaderMetrics sets all metrics that are emitted only on leaders to a NaN
// value so that they don't incorrectly report 0 when a server starts as a
// follower.
func initLeaderMetrics() {
	for _, g := range LeaderCertExpirationGauges {
		metrics.SetGaugeWithLabels(g.Name, float32(math.NaN()), g.ConstLabels)
	}
}
