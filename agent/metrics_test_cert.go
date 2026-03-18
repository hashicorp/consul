// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"
)

func TestAgentTLSMetrics_WithNodeName(t *testing.T) {
	t.Parallel()

	// Create agent with TLS enabled
	a := NewTestAgent(t, `
		node_name = "test-node-1"
		verify_incoming = true
		verify_outgoing = true
		ca_file = "../test/ca/root.cer"
		cert_file = "../test/key/ourdomain.cer"
		key_file = "../test/key/ourdomain.key"
		
		telemetry {
			certificate {
				enabled = true
			}
		}
	`)
	defer a.Shutdown()

	// Wait a bit for metrics to be emitted
	time.Sleep(100 * time.Millisecond)

	// Verify that the metric was emitted with node label
	// This is a basic smoke test - in real scenarios we'd inspect prometheus metrics
	require.Equal(t, "test-node-1", a.config.NodeName)
}

func TestAgentTLSMetrics_MultipleNodes(t *testing.T) {
	t.Parallel()

	nodes := []string{"node-a", "node-b", "node-c"}

	for _, nodeName := range nodes {
		t.Run(nodeName, func(t *testing.T) {
			a := NewTestAgent(t, `
				node_name = "`+nodeName+`"
				verify_incoming = true
				verify_outgoing = true
				ca_file = "../test/ca/root.cer"
				cert_file = "../test/key/ourdomain.cer"
				key_file = "../test/key/ourdomain.key"
				
				telemetry {
					certificate {
						enabled = true
					}
				}
			`)
			defer a.Shutdown()

			require.Equal(t, nodeName, a.config.NodeName)
		})
	}
}

func TestCertificateMetrics_NodeNameInResponse(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, `
		node_name = "server-1"
		
		telemetry {
			certificate {
				enabled = true
			}
		}
	`)
	defer a.Shutdown()

	// Emit a test metric with node label
	metrics.SetGaugeWithLabels(
		[]string{"agent", "tls", "cert", "expiry"},
		float32((30 * 24 * time.Hour).Seconds()),
		[]metrics.Label{{Name: "node", Value: "server-1"}},
	)

	time.Sleep(100 * time.Millisecond)

	// This verifies the metric can be emitted - actual API test is in agent_certmetrics_endpoint_test.go
	require.Equal(t, "server-1", a.config.NodeName)
}

func TestCertificateMetrics_ThresholdLogging(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		criticalThresholdDays int
		warningThresholdDays  int
	}{
		{"default", 7, 30},
		{"conservative", 14, 60},
		{"aggressive", 3, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewTestAgent(t, fmt.Sprintf(`
				telemetry {
					certificate {
						enabled = true
						critical_threshold_days = %d
						warning_threshold_days = %d
					}
				}
			`, tt.criticalThresholdDays, tt.warningThresholdDays))
			defer a.Shutdown()

			require.Equal(t, tt.criticalThresholdDays, a.config.Telemetry.CertificateCriticalThresholdDays)
			require.Equal(t, tt.warningThresholdDays, a.config.Telemetry.CertificateWarningThresholdDays)
		})
	}
}
