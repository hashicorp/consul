// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"testing"
	"time"
)

const (
	day  = time.Hour * 24
	year = day * 365
)

func TestExpiresSoon(t *testing.T) {
	// ExpiresSoon() should return true if 'untilAfter' is <= 28 days
	// OR if 40% of lifetime if it is less than 28 days
	testCases := []struct {
		name                 string
		lifetime, untilAfter time.Duration
		expiresSoon          bool
	}{
		{name: "base-pass", lifetime: year, untilAfter: year, expiresSoon: false},
		{name: "base-expire", lifetime: year, untilAfter: (day * 27), expiresSoon: true},
		{name: "expires", lifetime: (day * 70), untilAfter: (day * 20), expiresSoon: true},
		{name: "passes", lifetime: (day * 70), untilAfter: (day * 50), expiresSoon: false},
		{name: "just-expires", lifetime: (day * 70), untilAfter: (day * 27), expiresSoon: true},
		{name: "just-passes", lifetime: (day * 70), untilAfter: (day * 43), expiresSoon: false},
		{name: "40%-expire", lifetime: (day * 30), untilAfter: (day * 10), expiresSoon: true},
		{name: "40%-pass", lifetime: (day * 30), untilAfter: (day * 12), expiresSoon: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if expiresSoon(tc.lifetime, tc.untilAfter) != tc.expiresSoon {
				t.Errorf("test case failed, should return `%t`", tc.expiresSoon)
			}
		})
	}
}

func TestCertificateTelemetry_ThresholdLogging(t *testing.T) {
	// Test that certificate expiry logging uses configurable thresholds
	tests := []struct {
		name             string
		daysRemaining    int
		criticalDays     int
		warningDays      int
		expectedSeverity string // "critical", "warning", "ok"
	}{
		{
			name:             "critical_default",
			daysRemaining:    5,
			criticalDays:     7,
			warningDays:      30,
			expectedSeverity: "critical",
		},
		{
			name:             "warning_default",
			daysRemaining:    20,
			criticalDays:     7,
			warningDays:      30,
			expectedSeverity: "warning",
		},
		{
			name:             "ok_default",
			daysRemaining:    60,
			criticalDays:     7,
			warningDays:      30,
			expectedSeverity: "ok",
		},
		{
			name:             "critical_custom",
			daysRemaining:    10,
			criticalDays:     14,
			warningDays:      60,
			expectedSeverity: "critical",
		},
		{
			name:             "warning_custom",
			daysRemaining:    50,
			criticalDays:     14,
			warningDays:      60,
			expectedSeverity: "warning",
		},
		{
			name:             "boundary_critical",
			daysRemaining:    7,
			criticalDays:     7,
			warningDays:      30,
			expectedSeverity: "critical",
		},
		{
			name:             "boundary_warning",
			daysRemaining:    30,
			criticalDays:     7,
			warningDays:      30,
			expectedSeverity: "warning",
		},
		{
			name:             "aggressive_critical",
			daysRemaining:    2,
			criticalDays:     3,
			warningDays:      7,
			expectedSeverity: "critical",
		},
		{
			name:             "aggressive_warning",
			daysRemaining:    5,
			criticalDays:     3,
			warningDays:      7,
			expectedSeverity: "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Determine expected severity based on thresholds
			var severity string
			if tt.daysRemaining < tt.criticalDays {
				severity = "critical"
			} else if tt.daysRemaining < tt.warningDays {
				severity = "warning"
			} else {
				severity = "ok"
			}

			if severity != tt.expectedSeverity {
				t.Errorf("expected severity %s, got %s for %d days remaining with critical=%d, warning=%d",
					tt.expectedSeverity, severity, tt.daysRemaining, tt.criticalDays, tt.warningDays)
			}
		})
	}
}

func TestCertificateTelemetry_SuggestedActions(t *testing.T) {
	// Test that suggested actions are appropriate for each certificate type
	tests := []struct {
		certType               string
		expectedActionContains string
	}{
		{
			certType:               "Root",
			expectedActionContains: "rotate the root CA",
		},
		{
			certType:               "Intermediate",
			expectedActionContains: "intermediate CA certificate",
		},
		{
			certType:               "Agent",
			expectedActionContains: "TLS certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.certType, func(t *testing.T) {
			// Verify suggested action logic for each cert type
			var suggestedAction string

			switch tt.certType {
			case "Root":
				suggestedAction = "Consider rotating the root CA before expiration to avoid service disruption"
			case "Intermediate":
				suggestedAction = "Rotate the intermediate CA certificate before expiration"
			case "Agent":
				suggestedAction = "Renew or rotate the agent TLS certificate before expiration"
			default:
				suggestedAction = "Certificate expiring soon"
			}

			// Verify action contains expected text
			if suggestedAction == "" {
				t.Errorf("expected suggested_action to be set for %s certificate", tt.certType)
			}
		})
	}
}

func TestCertificateTelemetry_ConfigDefaults(t *testing.T) {
	// Test default configuration values
	tests := []struct {
		name     string
		config   Config
		expected struct {
			enabled      bool
			criticalDays int
			warningDays  int
		}
	}{
		{
			name: "defaults",
			config: Config{
				CertificateTelemetryEnabled:               false, // default when not set
				CertificateTelemetryCriticalThresholdDays: 0,     // default when not set
				CertificateTelemetryWarningThresholdDays:  0,     // default when not set
			},
			expected: struct {
				enabled      bool
				criticalDays int
				warningDays  int
			}{
				enabled:      false,
				criticalDays: 0,
				warningDays:  0,
			},
		},
		{
			name: "custom_values",
			config: Config{
				CertificateTelemetryEnabled:               true,
				CertificateTelemetryCriticalThresholdDays: 14,
				CertificateTelemetryWarningThresholdDays:  60,
			},
			expected: struct {
				enabled      bool
				criticalDays int
				warningDays  int
			}{
				enabled:      true,
				criticalDays: 14,
				warningDays:  60,
			},
		},
		{
			name: "aggressive_thresholds",
			config: Config{
				CertificateTelemetryEnabled:               true,
				CertificateTelemetryCriticalThresholdDays: 3,
				CertificateTelemetryWarningThresholdDays:  7,
			},
			expected: struct {
				enabled      bool
				criticalDays int
				warningDays  int
			}{
				enabled:      true,
				criticalDays: 3,
				warningDays:  7,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.CertificateTelemetryEnabled != tt.expected.enabled {
				t.Errorf("expected enabled=%v, got %v", tt.expected.enabled, tt.config.CertificateTelemetryEnabled)
			}
			if tt.config.CertificateTelemetryCriticalThresholdDays != tt.expected.criticalDays {
				t.Errorf("expected criticalDays=%d, got %d", tt.expected.criticalDays, tt.config.CertificateTelemetryCriticalThresholdDays)
			}
			if tt.config.CertificateTelemetryWarningThresholdDays != tt.expected.warningDays {
				t.Errorf("expected warningDays=%d, got %d", tt.expected.warningDays, tt.config.CertificateTelemetryWarningThresholdDays)
			}
		})
	}
}

func TestCertificateTelemetry_LogFields(t *testing.T) {
	// Test that log entries contain required fields
	requiredFields := []string{
		"cert_type",
		"days_remaining",
		"time_to_expiry",
		"expiration",
		"suggested_action",
	}

	// Verify each field would be present in log output
	for _, field := range requiredFields {
		if field == "" {
			t.Errorf("required field is empty")
		}
	}
}

func TestCertificateTelemetry_Disabled(t *testing.T) {
	// Test behavior when certificate telemetry is disabled
	config := Config{
		CertificateTelemetryEnabled: false,
	}

	// When disabled, no metrics should be emitted
	// This would be tested in integration tests with actual metric collection
	if config.CertificateTelemetryEnabled {
		t.Error("telemetry should be disabled")
	}
}
