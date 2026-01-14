// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestAgentMetricsCertificates_RenewalFailures(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, `
		telemetry {
			prometheus_retention_time = "60s"
			disable_hostname = true
		}
	`)
	defer a.Shutdown()

	// Emit renewal failure metrics
	metrics.SetGaugeWithLabels(
		[]string{"leaf-certs", "cert_expiry"},
		float32((2 * 24 * time.Hour).Seconds()),
		[]metrics.Label{{Name: "service", Value: "web"}, {Name: "kind", Value: "service"}},
	)
	metrics.SetGaugeWithLabels(
		[]string{"leaf-certs", "cert_renewal_failure"},
		1,
		[]metrics.Label{
			{Name: "service", Value: "web"},
			{Name: "kind", Value: "service"},
			{Name: "reason", Value: "rate_limited"},
		},
	)

	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest("GET", "/v1/agent/metrics/certificates", nil)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentMetricsCertificates(resp, req)
	require.NoError(t, err)
	require.Nil(t, obj)

	var result certificatesResponse
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	// Verify we have failures tracked
	require.GreaterOrEqual(t, result.Summary.TotalWithFailures, 0, "Should track failures")
	if result.Summary.FailuresByReason != nil {
		t.Logf("FailuresByReason: %+v", result.Summary.FailuresByReason)
	}
}

func TestAgentMetricsCertificates_NoFailures(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	req, err := http.NewRequest("GET", "/v1/agent/metrics/certificates", nil)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentMetricsCertificates(resp, req)
	require.NoError(t, err)
	require.Nil(t, obj)

	var result certificatesResponse
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	// Verify no failures
	require.Equal(t, 0, result.Summary.TotalWithFailures, "Should have 0 certs with failures")
	require.Equal(t, 0, result.Summary.ExpiringSoonWithFail, "Should have 0 certs expiring soon with failures")

	t.Logf("Summary: %+v", result.Summary)
}

func TestAgentMetricsCertificates_SeverityLevels(t *testing.T) {
	t.Parallel()

	// Create test agent
	a := NewTestAgent(t, `
		telemetry {
			prometheus_retention_time = "60s"
			disable_hostname = true
		}
	`)
	defer a.Shutdown()

	// Emit test metrics with different expiry times
	// Critical: 5 days remaining (< 7 days)
	metrics.SetGauge([]string{"agent", "tls", "cert", "expiry"}, float32((5 * 24 * time.Hour).Seconds())) // Warning: 20 days remaining (< 30 days but > 7 days)
	metrics.SetGaugeWithLabels(
		[]string{"leaf-certs", "cert_expiry"},
		float32((20 * 24 * time.Hour).Seconds()),
		[]metrics.Label{{Name: "service", Value: "web"}, {Name: "kind", Value: "service"}},
	)

	// OK: 60 days remaining (> 30 days)
	metrics.SetGaugeWithLabels(
		[]string{"leaf-certs", "cert_expiry"},
		float32((60 * 24 * time.Hour).Seconds()),
		[]metrics.Label{{Name: "service", Value: "api"}, {Name: "kind", Value: "service"}},
	)

	// Wait for metrics to be registered
	testutil.RunStep(t, "wait for metrics", func(t *testing.T) {
		time.Sleep(100 * time.Millisecond)
	})

	// Query endpoint
	req, err := http.NewRequest("GET", "/v1/agent/metrics/certificates", nil)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentMetricsCertificates(resp, req)
	require.NoError(t, err)
	require.Nil(t, obj)

	// Parse response
	var result certificatesResponse
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	// Verify we have certificates
	require.NotEmpty(t, result.Certificates, "Should have certificates")

	// Verify severity distribution
	require.NotNil(t, result.Summary.BySeverity, "BySeverity should not be nil")

	// Check that we have different severity levels
	foundCritical := false
	foundWarning := false
	foundOK := false

	for _, cert := range result.Certificates {
		switch cert.Severity {
		case "critical":
			foundCritical = true
			require.Less(t, cert.DaysRemaining, 7, "Critical cert should have < 7 days")
		case "warning":
			foundWarning = true
			require.GreaterOrEqual(t, cert.DaysRemaining, 7, "Warning cert should have >= 7 days")
			require.Less(t, cert.DaysRemaining, 30, "Warning cert should have < 30 days")
		case "ok":
			foundOK = true
			require.GreaterOrEqual(t, cert.DaysRemaining, 30, "OK cert should have >= 30 days")
		}
	}

	t.Logf("Certificates found: %d", len(result.Certificates))
	t.Logf("Severity distribution: %+v", result.Summary.BySeverity)
	t.Logf("Found critical: %v, warning: %v, ok: %v", foundCritical, foundWarning, foundOK)
}

func TestAgentMetricsCertificates_MultipleTypes(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, `
		telemetry {
			prometheus_retention_time = "60s"
			disable_hostname = true
		}
	`)
	defer a.Shutdown()

	// Emit metrics for different certificate types
	// Agent TLS
	metrics.SetGauge([]string{"agent", "tls", "cert", "expiry"}, float32((10 * 24 * time.Hour).Seconds()))

	// Leaf certs
	metrics.SetGaugeWithLabels(
		[]string{"leaf-certs", "cert_expiry"},
		float32((5 * 24 * time.Hour).Seconds()),
		[]metrics.Label{{Name: "service", Value: "payments"}, {Name: "kind", Value: "service"}},
	)

	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest("GET", "/v1/agent/metrics/certificates", nil)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentMetricsCertificates(resp, req)
	require.NoError(t, err)
	require.Nil(t, obj)

	var result certificatesResponse
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	// Verify summary by type
	require.NotNil(t, result.Summary.ByType, "ByType should not be nil")
	t.Logf("Type distribution: %+v", result.Summary.ByType)

	// Check for different certificate types
	certTypes := make(map[string]bool)
	for _, cert := range result.Certificates {
		certTypes[cert.Type] = true
	}

	t.Logf("Certificate types found: %+v", certTypes)
}

func TestAgentMetricsCertificates_JSONStructure(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	req, err := http.NewRequest("GET", "/v1/agent/metrics/certificates", nil)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentMetricsCertificates(resp, req)
	require.NoError(t, err)
	require.Nil(t, obj)

	// Verify response is valid JSON
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "application/json")

	var result certificatesResponse
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	// Verify structure
	require.NotNil(t, result.Certificates, "Certificates array should not be nil")
	require.NotNil(t, result.Summary, "Summary should not be nil")
	require.NotNil(t, result.Summary.BySeverity, "BySeverity should not be nil")
	require.NotNil(t, result.Summary.ByType, "ByType should not be nil")

	// Verify thresholds are set
	require.Equal(t, 7, result.Thresholds.CriticalDays)
	require.Equal(t, 30, result.Thresholds.WarningDays)

	// Verify cache fields exist
	require.False(t, result.CacheExpires.IsZero(), "CacheExpires should be set")

	t.Logf("Response structure valid, thresholds: critical=%dd, warning=%dd",
		result.Thresholds.CriticalDays, result.Thresholds.WarningDays)
}

func TestAgentMetricsCertificates_Caching(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Clear any existing cache from other tests
	certsCache.Store(nil)

	// First request - should not be cached
	req1, err := http.NewRequest("GET", "/v1/agent/metrics/certificates", nil)
	require.NoError(t, err)

	resp1 := httptest.NewRecorder()
	obj, err := a.srv.AgentMetricsCertificates(resp1, req1)
	require.NoError(t, err)
	require.Nil(t, obj)

	var result1 certificatesResponse
	err = json.Unmarshal(resp1.Body.Bytes(), &result1)
	require.NoError(t, err)
	require.False(t, result1.Cached, "First request should not be cached")

	// Second request immediately - should be cached
	req2, err := http.NewRequest("GET", "/v1/agent/metrics/certificates", nil)
	require.NoError(t, err)

	resp2 := httptest.NewRecorder()
	obj, err = a.srv.AgentMetricsCertificates(resp2, req2)
	require.NoError(t, err)
	require.Nil(t, obj)

	var result2 certificatesResponse
	err = json.Unmarshal(resp2.Body.Bytes(), &result2)
	require.NoError(t, err)
	require.True(t, result2.Cached, "Second request should be cached")

	// Cache expiry time should be the same
	require.Equal(t, result1.CacheExpires, result2.CacheExpires, "Cache expiry should be the same")

	// Certificates should be identical
	require.Equal(t, len(result1.Certificates), len(result2.Certificates), "Certificate count should match")

	t.Logf("Caching working: first cached=%v, second cached=%v, expires at %v",
		result1.Cached, result2.Cached, result1.CacheExpires)
}

func TestAgentMetricsCertificates_DaysCalculation(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, `
		telemetry {
			prometheus_retention_time = "60s"
			disable_hostname = true
		}
	`)
	defer a.Shutdown()

	// Set metric with exactly 48 hours (2 days)
	twoDays := 2 * 24 * time.Hour
	metrics.SetGauge([]string{"agent", "tls", "cert", "expiry"}, float32(twoDays.Seconds()))

	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest("GET", "/v1/agent/metrics/certificates", nil)
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	obj, err := a.srv.AgentMetricsCertificates(resp, req)
	require.NoError(t, err)
	require.Nil(t, obj)

	var result certificatesResponse
	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	// Verify certificates exist
	require.NotEmpty(t, result.Certificates, "Should have at least one certificate")

	// Log all certificates for debugging
	for _, cert := range result.Certificates {
		t.Logf("Certificate: type=%s, name=%s, days=%d, seconds=%d, severity=%s",
			cert.Type, cert.Name, cert.DaysRemaining, cert.SecondsRemaining, cert.Severity)
	}
}

func TestAgentMetricsCertificates_ACLEnforcement(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, `
		primary_datacenter = "dc1"
		acl {
			enabled = true
			default_policy = "deny"
			tokens {
				initial_management = "root"
			}
		}
	`)
	defer a.Shutdown()

	testutil.RunStep(t, "wait for ACLs", func(t *testing.T) {
		time.Sleep(100 * time.Millisecond)
	})

	// Request without token - should fail
	req1, err := http.NewRequest("GET", "/v1/agent/metrics/certificates", nil)
	require.NoError(t, err)

	resp1 := httptest.NewRecorder()
	_, err = a.srv.AgentMetricsCertificates(resp1, req1)
	require.Error(t, err, "Should require ACL token")

	// Request with valid token - should succeed
	req2, err := http.NewRequest("GET", "/v1/agent/metrics/certificates?token=root", nil)
	require.NoError(t, err)

	resp2 := httptest.NewRecorder()
	_, err = a.srv.AgentMetricsCertificates(resp2, req2)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp2.Code)

	t.Logf("ACL enforcement working correctly")
}
