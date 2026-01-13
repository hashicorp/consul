// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	promcollect "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
)

// cachedCertsResult caches the response payload and expiry time
type cachedCertsResult struct {
	payload   certificatesResponse
	expiresAt time.Time
}

// certsCache holds an atomic pointer to cachedCertsResult
var certsCache atomic.Pointer[cachedCertsResult]

type certificateItem struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Type                   string `json:"type"`
	Subtype                string `json:"subtype,omitempty"`
	Subject                string `json:"subject,omitempty"`
	Issuer                 string `json:"issuer,omitempty"`
	NotAfter               string `json:"not_after,omitempty"`
	DaysRemaining          int    `json:"days_remaining"`
	SecondsRemaining       int64  `json:"seconds_remaining"`
	Severity               string `json:"severity"`
	SeverityScore          int    `json:"severity_score"`
	Color                  string `json:"color"`
	AutoRenewable          bool   `json:"auto_renewable,omitempty"`
	RequiresManualRotation bool   `json:"requires_manual_rotation,omitempty"`
	CAProvider             string `json:"ca_provider,omitempty"`
	Path                   string `json:"path,omitempty"`
	// Renewal failure tracking
	RenewalFailure *certificateRenewalFailure `json:"renewal_failure,omitempty"`
}

type certificateRenewalFailure struct {
	FailureReason         string    `json:"failure_reason"`
	FailureCount          int       `json:"failure_count"`
	LastAttempt           time.Time `json:"last_attempt"`
	ConsecutiveRateLimits int       `json:"consecutive_rate_limits"`
}

type certificatesSummary struct {
	Total      int            `json:"total"`
	BySeverity map[string]int `json:"by_severity"`
	ByType     map[string]int `json:"by_type"`
	// Renewal failure summary
	TotalWithFailures    int            `json:"total_with_failures"`
	ExpiringSoonWithFail int            `json:"expiring_soon_with_failures"`
	FailuresByReason     map[string]int `json:"failures_by_reason,omitempty"`
}

type certificatesResponse struct {
	Certificates []certificateItem   `json:"certificates"`
	Summary      certificatesSummary `json:"summary"`
	Thresholds   struct {
		CriticalDays int `json:"critical_days"`
		WarningDays  int `json:"warning_days"`
	} `json:"thresholds"`
	Cached       bool      `json:"cached"`
	CacheExpires time.Time `json:"cache_expires_at"`
}

// thresholds
const (
	criticalDays = 7
	warningDays  = 30
	cacheTTL     = 5 * time.Minute
)

// metricMapping defines how to identify and categorize certificate expiry metrics
type metricMapping struct {
	suffix      string // metric name suffix to match (e.g., "mesh_active_root_ca_expiry")
	id          string // unique identifier for this cert type
	displayName string // human-readable name
	certType    string // certificate type (ca, agent, leaf, gateway)
	subtype     string // certificate subtype (root, intermediate, service, mesh, etc.)
}

// certificateMetricMappings defines all known certificate expiry metrics
// Add new entries here to automatically support new metrics
var certificateMetricMappings = []metricMapping{
	{
		suffix:      "mesh_active_root_ca_expiry",
		id:          "ca-root-primary",
		displayName: "Root CA",
		certType:    "ca",
		subtype:     "root",
	},
	{
		suffix:      "mesh_active_signing_ca_expiry",
		id:          "ca-signing-active",
		displayName: "Active Signing CA",
		certType:    "ca",
		subtype:     "intermediate",
	},
	{
		suffix:      "agent_tls_cert_expiry",
		id:          "agent-tls",
		displayName: "Agent TLS Certificate",
		certType:    "agent",
		subtype:     "server",
	},
	{
		suffix:      "leaf_certs_cert_expiry",
		id:          "leaf-cert",
		displayName: "Leaf Certificate",
		certType:    "leaf",
		subtype:     "service",
	},
}

// AgentMetricsCertificates provides a summarized view of certificate expiry using agent metrics.
func (s *HTTPHandlers) AgentMetricsCertificates(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// ACL: require AgentRead similar to AgentMetrics
	var token string
	s.parseToken(req, &token)
	authz, err := s.agent.delegate.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, err
	}

	var authzContext = acl.AuthorizerContext{}
	s.agent.AgentEnterpriseMeta().FillAuthzContext(&authzContext)
	if err := authz.ToAllowAuthorizer().AgentReadAllowed(s.agent.config.NodeName, &authzContext); err != nil {
		return nil, err
	}

	// Serve from cache if valid
	if cached := certsCache.Load(); cached != nil && time.Now().Before(cached.expiresAt) {
		// Make a copy and mark as cached to avoid mutating the stored payload
		payload := cached.payload
		payload.Cached = true

		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		resp.Header().Set(contentTypeHeader, api.GetContentType(req))
		resp.WriteHeader(http.StatusOK)
		_, _ = resp.Write(data)
		return nil, nil
	}

	// Gather metrics from the default gatherer
	fams, err := promcollect.DefaultGatherer.Gather()
	if err != nil {
		return nil, err
	}

	items := extractCertificateItemsFromFamilies(fams)
	summary := summarizeCertificateItems(items)

	payload := certificatesResponse{
		Certificates: items,
		Summary:      summary,
		Cached:       false,
	}
	payload.Thresholds.CriticalDays = criticalDays
	payload.Thresholds.WarningDays = warningDays
	payload.CacheExpires = time.Now().Add(cacheTTL)

	// Cache the payload structure (not JSON bytes)
	certsCache.Store(&cachedCertsResult{payload: payload, expiresAt: payload.CacheExpires})

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp.Header().Set(contentTypeHeader, api.GetContentType(req))
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write(data)
	return nil, nil
}

func extractCertificateItemsFromFamilies(fams []*dto.MetricFamily) []certificateItem {
	var items []certificateItem

	// First pass: collect all failure info by service+kind
	failureMap := make(map[string]*certificateRenewalFailure)
	for _, fam := range fams {
		if fam == nil || fam.Name == nil {
			continue
		}

		// Look for renewal failure metrics
		if hasSuffix(fam.GetName(), "leaf_certs_cert_renewal_failure") {
			for _, m := range fam.GetMetric() {
				if m.Gauge != nil && m.Gauge.Value != nil && *m.Gauge.Value > 0 {
					labels := make(map[string]string)
					for _, label := range m.Label {
						if label.Name != nil && label.Value != nil {
							labels[*label.Name] = *label.Value
						}
					}

					service := labels["service"]
					kind := labels["kind"]
					reason := labels["reason"]
					key := service + "|" + kind

					failureMap[key] = &certificateRenewalFailure{
						FailureReason: reason,
						// We don't have count/lastAttempt in metrics yet, could add later
					}
				}
			}
		}
	}

	// Second pass: build certificate items
	for _, fam := range fams {
		if fam == nil || fam.Name == nil {
			continue
		}
		name := fam.GetName()

		// Try to match against known metric mappings
		for _, mapping := range certificateMetricMappings {
			if hasSuffix(name, mapping.suffix) {
				items = append(items, buildItemFromFamily(
					fam,
					mapping.id,
					mapping.displayName,
					mapping.certType,
					mapping.subtype,
					failureMap,
				)...)
				break // matched, no need to check other mappings
			}
		}
	}
	return items
}

func hasSuffix(name, suffix string) bool {
	if name == suffix {
		return true
	}
	return strings.HasSuffix(name, "_"+suffix)
}

func buildItemFromFamily(fam *dto.MetricFamily, id, displayName, typ, subtype string, failureMap map[string]*certificateRenewalFailure) []certificateItem {
	var out []certificateItem
	for _, m := range fam.GetMetric() {
		seconds := int64(0)
		if m.Gauge != nil && m.Gauge.Value != nil {
			// prometheus client reports float64
			v := *m.Gauge.Value
			if math.IsNaN(v) || math.IsInf(v, 0) {
				continue
			}
			seconds = int64(v)
		}
		days := int(seconds / 86400)
		severity, color, score := severityForDays(days)

		// Extract labels for more specific identification
		itemID := id
		itemName := displayName
		itemSubtype := subtype
		var failureKey string

		// For metrics with labels (e.g., leaf certs), use labels to create unique IDs
		if len(m.Label) > 0 {
			labels := make(map[string]string)
			for _, label := range m.Label {
				if label.Name != nil && label.Value != nil {
					labels[*label.Name] = *label.Value
				}
			}

			// For leaf certs, use service name and kind
			if serviceName, ok := labels["service"]; ok {
				itemID = serviceName
				itemName = serviceName
				kind := labels["kind"]
				failureKey = serviceName + "|" + kind

				// Determine subtype based on kind label
				if kind != "" {
					switch kind {
					case "mesh-gateway":
						itemSubtype = "mesh"
						itemName = "Mesh Gateway - " + serviceName
					case "ingress-gateway":
						itemSubtype = "ingress"
						itemName = "Ingress Gateway - " + serviceName
					case "terminating-gateway":
						itemSubtype = "terminating"
						itemName = "Terminating Gateway - " + serviceName
					case "api-gateway":
						itemSubtype = "api"
						itemName = "API Gateway - " + serviceName
					default:
						itemSubtype = "service"
					}
				}
			}
		}

		item := certificateItem{
			ID:               itemID,
			Name:             itemName,
			Type:             typ,
			Subtype:          itemSubtype,
			DaysRemaining:    days,
			SecondsRemaining: seconds,
			Severity:         severity,
			SeverityScore:    score,
			Color:            color,
		}

		// Add failure info if available
		if failureKey != "" {
			if failure, ok := failureMap[failureKey]; ok {
				item.RenewalFailure = failure
			}
		}

		out = append(out, item)
	}
	return out
}

func severityForDays(days int) (severity, color string, score int) {
	switch {
	case days < criticalDays:
		return "CRITICAL", "red", 90
	case days < warningDays:
		return "WARNING", "yellow", 50
	default:
		return "INFO", "green", 10
	}
}

func summarizeCertificateItems(items []certificateItem) certificatesSummary {
	s := certificatesSummary{
		Total:            len(items),
		BySeverity:       map[string]int{"critical": 0, "warning": 0, "info": 0},
		ByType:           map[string]int{},
		FailuresByReason: map[string]int{},
	}

	for _, it := range items {
		switch it.Severity {
		case "CRITICAL":
			s.BySeverity["critical"]++
		case "WARNING":
			s.BySeverity["warning"]++
		default:
			s.BySeverity["info"]++
		}
		s.ByType[it.Type] = s.ByType[it.Type] + 1

		// Track renewal failures
		if it.RenewalFailure != nil {
			s.TotalWithFailures++
			s.FailuresByReason[it.RenewalFailure.FailureReason]++

			// Track expiring soon with failures (critical or warning)
			if it.Severity == "CRITICAL" || it.Severity == "WARNING" {
				s.ExpiringSoonWithFail++
			}
		}
	}
	return s
}
