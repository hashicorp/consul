// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCertificateTelemetry_Defaults(t *testing.T) {
	// Test that default values are applied when no certificate telemetry config is provided
	hcl := `
		data_dir = "/tmp/consul"
		bind_addr = "127.0.0.1"
	`

	result, err := Load(LoadOpts{HCL: []string{hcl}})
	require.NoError(t, err)
	require.NotNil(t, result.RuntimeConfig)

	rt := result.RuntimeConfig

	// Verify defaults
	require.True(t, rt.Telemetry.CertificateEnabled, "Certificate telemetry should be enabled by default")
	require.Equal(t, 5*time.Minute, rt.Telemetry.CertificateCacheDuration, "Default cache duration should be 5 minutes")
	require.Equal(t, 7, rt.Telemetry.CertificateCriticalThresholdDays, "Default critical threshold should be 7 days")
	require.Equal(t, 30, rt.Telemetry.CertificateWarningThresholdDays, "Default warning threshold should be 30 days")
	require.Equal(t, 90, rt.Telemetry.CertificateInfoThresholdDays, "Default info threshold should be 90 days")
	require.False(t, rt.Telemetry.CertificateExcludeAutoRenewable, "Exclude auto-renewable should be false by default")
}

func TestCertificateTelemetry_CustomValues(t *testing.T) {
	// Test that custom values override defaults
	hcl := `
		data_dir = "/tmp/consul"
		bind_addr = "127.0.0.1"
		
		telemetry {
			certificate {
				enabled = false
				cache_duration = "10m"
				critical_threshold_days = 14
				warning_threshold_days = 60
				info_threshold_days = 180
				exclude_auto_renewable = true
			}
		}
	`

	result, err := Load(LoadOpts{HCL: []string{hcl}})
	require.NoError(t, err)
	require.NotNil(t, result.RuntimeConfig)

	rt := result.RuntimeConfig

	// Verify custom values
	require.False(t, rt.Telemetry.CertificateEnabled, "Certificate telemetry should be disabled")
	require.Equal(t, 10*time.Minute, rt.Telemetry.CertificateCacheDuration, "Cache duration should be 10 minutes")
	require.Equal(t, 14, rt.Telemetry.CertificateCriticalThresholdDays, "Critical threshold should be 14 days")
	require.Equal(t, 60, rt.Telemetry.CertificateWarningThresholdDays, "Warning threshold should be 60 days")
	require.Equal(t, 180, rt.Telemetry.CertificateInfoThresholdDays, "Info threshold should be 180 days")
	require.True(t, rt.Telemetry.CertificateExcludeAutoRenewable, "Exclude auto-renewable should be true")
}

func TestCertificateTelemetry_PartialConfig(t *testing.T) {
	// Test that partial config merges with defaults
	hcl := `
		data_dir = "/tmp/consul"
		bind_addr = "127.0.0.1"
		
		telemetry {
			certificate {
				critical_threshold_days = 3
				warning_threshold_days = 7
			}
		}
	`

	result, err := Load(LoadOpts{HCL: []string{hcl}})
	require.NoError(t, err)
	require.NotNil(t, result.RuntimeConfig)

	rt := result.RuntimeConfig

	// Verify partial override with defaults
	require.True(t, rt.Telemetry.CertificateEnabled, "Should use default (true)")
	require.Equal(t, 5*time.Minute, rt.Telemetry.CertificateCacheDuration, "Should use default (5m)")
	require.Equal(t, 3, rt.Telemetry.CertificateCriticalThresholdDays, "Should use custom value (3)")
	require.Equal(t, 7, rt.Telemetry.CertificateWarningThresholdDays, "Should use custom value (7)")
	require.Equal(t, 90, rt.Telemetry.CertificateInfoThresholdDays, "Should use default (90)")
	require.False(t, rt.Telemetry.CertificateExcludeAutoRenewable, "Should use default (false)")
}

func TestCertificateTelemetry_DurationParsing(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		expected time.Duration
	}{
		{"seconds", "30s", 30 * time.Second},
		{"minutes", "15m", 15 * time.Minute},
		{"hours", "2h", 2 * time.Hour},
		{"combined", "1h30m", 90 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hcl := `
				data_dir = "/tmp/consul"
				bind_addr = "127.0.0.1"
				
				telemetry {
					certificate {
						cache_duration = "` + tt.duration + `"
					}
				}
			`

			result, err := Load(LoadOpts{HCL: []string{hcl}})
			require.NoError(t, err)
			require.Equal(t, tt.expected, result.RuntimeConfig.Telemetry.CertificateCacheDuration)
		})
	}
}

func TestCertificateTelemetry_InvalidDuration(t *testing.T) {
	hcl := `
		data_dir = "/tmp/consul"
		bind_addr = "127.0.0.1"
		
		telemetry {
			certificate {
				cache_duration = "invalid"
			}
		}
	`

	_, err := Load(LoadOpts{HCL: []string{hcl}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "telemetry.certificate.cache_duration")
}

func TestCertificateTelemetry_ThresholdValidation(t *testing.T) {
	// Test that thresholds can be set to various values including edge cases
	tests := []struct {
		name     string
		critical int
		warning  int
		info     int
	}{
		{"normal", 7, 30, 90},
		{"conservative", 14, 60, 180},
		{"aggressive", 1, 3, 7},
		{"zero_critical", 0, 10, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hcl := fmt.Sprintf(`
				data_dir = "/tmp/consul"
				bind_addr = "127.0.0.1"
				
				telemetry {
					certificate {
						critical_threshold_days = %d
						warning_threshold_days = %d
						info_threshold_days = %d
					}
				}
			`, tt.critical, tt.warning, tt.info)

			result, err := Load(LoadOpts{HCL: []string{hcl}})
			require.NoError(t, err)
			require.Equal(t, tt.critical, result.RuntimeConfig.Telemetry.CertificateCriticalThresholdDays)
			require.Equal(t, tt.warning, result.RuntimeConfig.Telemetry.CertificateWarningThresholdDays)
			require.Equal(t, tt.info, result.RuntimeConfig.Telemetry.CertificateInfoThresholdDays)
		})
	}
}

func TestCertificateTelemetry_JSONConfig(t *testing.T) {
	// Test JSON configuration format
	json := `{
		"data_dir": "/tmp/consul",
		"bind_addr": "127.0.0.1",
		"telemetry": {
			"certificate": {
				"enabled": false,
				"cache_duration": "15m",
				"critical_threshold_days": 5,
				"warning_threshold_days": 20,
				"info_threshold_days": 60,
				"exclude_auto_renewable": true
			}
		}
	}`

	result, err := Load(LoadOpts{HCL: []string{json}})
	require.NoError(t, err)
	require.NotNil(t, result.RuntimeConfig)

	rt := result.RuntimeConfig
	require.False(t, rt.Telemetry.CertificateEnabled)
	require.Equal(t, 15*time.Minute, rt.Telemetry.CertificateCacheDuration)
	require.Equal(t, 5, rt.Telemetry.CertificateCriticalThresholdDays)
	require.Equal(t, 20, rt.Telemetry.CertificateWarningThresholdDays)
	require.Equal(t, 60, rt.Telemetry.CertificateInfoThresholdDays)
	require.True(t, rt.Telemetry.CertificateExcludeAutoRenewable)
}

func TestCertificateTelemetry_MultipleConfigSources(t *testing.T) {
	// Test that later configs override earlier ones
	hcl1 := `
		data_dir = "/tmp/consul"
		bind_addr = "127.0.0.1"
		
		telemetry {
			certificate {
				critical_threshold_days = 7
				warning_threshold_days = 30
			}
		}
	`

	hcl2 := `
		telemetry {
			certificate {
				critical_threshold_days = 14
			}
		}
	`

	result, err := Load(LoadOpts{HCL: []string{hcl1, hcl2}})
	require.NoError(t, err)
	require.NotNil(t, result.RuntimeConfig)

	rt := result.RuntimeConfig
	require.Equal(t, 14, rt.Telemetry.CertificateCriticalThresholdDays, "Should use value from second config")
	require.Equal(t, 30, rt.Telemetry.CertificateWarningThresholdDays, "Should keep value from first config")
}

func TestCertificateTelemetry_ConsulServerConfig(t *testing.T) {
	// Test that telemetry config is properly passed to consul server config
	hcl := `
		data_dir = "/tmp/consul"
		bind_addr = "127.0.0.1"
		server = true
		bootstrap = true
		
		telemetry {
			certificate {
				enabled = false
				critical_threshold_days = 10
				warning_threshold_days = 40
			}
		}
	`

	result, err := Load(LoadOpts{HCL: []string{hcl}})
	require.NoError(t, err)
	require.NotNil(t, result.RuntimeConfig)

	rt := result.RuntimeConfig
	require.False(t, rt.Telemetry.CertificateEnabled)
	require.Equal(t, 10, rt.Telemetry.CertificateCriticalThresholdDays)
	require.Equal(t, 40, rt.Telemetry.CertificateWarningThresholdDays)
}
