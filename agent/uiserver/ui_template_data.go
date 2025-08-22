// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package uiserver

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/hashicorp/consul/agent/config"
)

// sanitizeContentPath validates and sanitizes the content path to prevent XSS
// It ensures the path only contains safe characters and has proper structure
func sanitizeContentPath(path string) (string, error) {
	if path == "" {
		return "/", nil
	}

	// Only allow alphanumeric characters, hyphens, underscores, forward slashes, and dots
	validPathRegex := regexp.MustCompile(`^[a-zA-Z0-9/_.-]+$`)
	if !validPathRegex.MatchString(path) {
		return "", fmt.Errorf("contentPath contains invalid characters: %s", path)
	}

	// Ensure path starts and ends with forward slash for proper URL structure
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	// Additional validation: parse as URL to catch any malicious patterns
	if _, err := url.Parse(path); err != nil {
		return "", fmt.Errorf("contentPath is not a valid URL path: %s", path)
	}

	return path, nil
}

// sanitizeString removes or escapes potentially dangerous characters from strings
// This is a defense-in-depth measure for string values that might end up in templates
func sanitizeString(s string) string {
	// Remove or escape characters that could be used in XSS attacks
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#x27;")
	s = strings.ReplaceAll(s, "&", "&amp;") // Do this last to avoid double-escaping
	return s
}

// validateJSONContent validates that JSON content is safe
func validateJSONContent(jsonStr string) error {
	// Check if the JSON is valid
	var temp interface{}
	if err := json.Unmarshal([]byte(jsonStr), &temp); err != nil {
		return fmt.Errorf("invalid JSON content: %w", err)
	}

	// Check for potentially dangerous patterns in JSON strings
	dangerousPatterns := []string{"<script", "javascript:", "data:text/html", "vbscript:", "onload=", "onerror="}
	lowerJSON := strings.ToLower(jsonStr)

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerJSON, pattern) {
			return fmt.Errorf("JSON content contains potentially dangerous pattern: %s", pattern)
		}
	}

	return nil
}

// uiTemplateDataFromConfig returns the base set of variables that should be
// injected into the UI's Env based on the given runtime UI config.
// This function includes input validation and sanitization to prevent XSS attacks.
func uiTemplateDataFromConfig(cfg *config.RuntimeConfig) (map[string]interface{}, error) {

	// Sanitize and validate the content path
	contentPath, err := sanitizeContentPath(cfg.UIConfig.ContentPath)
	if err != nil {
		return nil, fmt.Errorf("invalid UI content path: %w", err)
	}

	// Sanitize string values that could potentially contain user input
	metricsProvider := sanitizeString(cfg.UIConfig.MetricsProvider)

	uiCfg := map[string]interface{}{
		"metrics_provider": metricsProvider,
		// We explicitly MUST NOT pass the metrics_proxy object since it might
		// contain add_headers with secrets that the UI shouldn't know e.g. API
		// tokens for the backend. The provider should either require the proxy to
		// be configured and then use that or hit the backend directly from the
		// browser.
		"metrics_proxy_enabled":   cfg.UIConfig.MetricsProxy.BaseURL != "",
		"dashboard_url_templates": cfg.UIConfig.DashboardURLTemplates,
		"hcp_enabled":             cfg.UIConfig.HCPEnabled,
	}

	// Validate JSON content for metrics provider options
	if cfg.UIConfig.MetricsProviderOptionsJSON != "" {
		if err := validateJSONContent(cfg.UIConfig.MetricsProviderOptionsJSON); err != nil {
			return nil, fmt.Errorf("invalid metrics provider options JSON: %w", err)
		}
		uiCfg["metrics_provider_options"] = json.RawMessage(cfg.UIConfig.MetricsProviderOptionsJSON)
	}

	// Sanitize datacenter names (defense in depth)
	localDatacenter := sanitizeString(cfg.Datacenter)
	primaryDatacenter := sanitizeString(cfg.PrimaryDatacenter)

	d := map[string]interface{}{
		"ContentPath":       contentPath,
		"ACLsEnabled":       cfg.ACLsEnabled,
		"HCPEnabled":        cfg.UIConfig.HCPEnabled,
		"UIConfig":          uiCfg,
		"LocalDatacenter":   localDatacenter,
		"PrimaryDatacenter": primaryDatacenter,
		"PeeringEnabled":    cfg.PeeringEnabled,
	}

	// Validate and sanitize extra script paths
	if len(cfg.UIConfig.MetricsProviderFiles) > 0 {
		extraScriptPath := contentPath + compiledProviderJSPath
		// Additional validation for the script path
		if _, err := url.Parse(extraScriptPath); err != nil {
			return nil, fmt.Errorf("invalid extra script path: %w", err)
		}
		d["ExtraScripts"] = []string{extraScriptPath}
	}

	return d, nil
}
