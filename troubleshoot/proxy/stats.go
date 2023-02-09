package troubleshoot

import (
	"encoding/json"
	"fmt"
	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"github.com/hashicorp/consul/troubleshoot/validate"
	"strings"
)

type statsJson struct {
	Stats []simpleMetric `json:"stats"`
}

type simpleMetric struct {
	Value int64  `json:"value,omitempty"`
	Name  string `json:"name,omitempty"`
}

func (t *Troubleshoot) troubleshootStats(upstreamEnvoyID string) (validate.Messages, error) {

	statMessages := validate.Messages{}
	envoyId := strings.Split(upstreamEnvoyID, "?")

	rejectionStats, err := t.getEnvoyStats(envoyId[0] + ".*update_rejected")
	if err != nil {
		return nil, fmt.Errorf("could not get config rejection stats from envoy admin API: %w", err)
	}

	for _, rs := range rejectionStats {
		if rs.Value != 0 {
			msg := validate.Message{
				Success: true,
				Message: fmt.Sprintf("Upstream has %v rejected configurations", rs.Value),
			}
			statMessages = append(statMessages, msg)
		}
	}

	connectionFailureStats, err := t.getEnvoyStats(envoyId[0] + ".*upstream_cx_connect_fail")
	if err != nil {
		return nil, fmt.Errorf("could not get connection failure stats from envoy admin API: %w", err)
	}
	for _, cfs := range connectionFailureStats {
		if cfs.Value != 0 {
			msg := validate.Message{
				Success: true,
				Message: fmt.Sprintf("upstream has %v connection failure(s).", cfs.Value),
			}
			statMessages = append(statMessages, msg)
		}
	}
	return statMessages, nil
}

func (t *Troubleshoot) getEnvoyStats(filter string) ([]*envoy_admin_v3.SimpleMetric, error) {

	var resultErr error

	jsonRaw, err := t.request(fmt.Sprintf("stats?format=json&filter=%s&type=Counters", filter))
	if err != nil {
		return nil, fmt.Errorf("error in requesting envoy Admin API /stats endpoint: %w", err)
	}

	var rawStats statsJson

	err = json.Unmarshal(jsonRaw, &rawStats)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal /stats response: %w", err)
	}

	stats := []*envoy_admin_v3.SimpleMetric{}

	for _, s := range rawStats.Stats {
		stat := &envoy_admin_v3.SimpleMetric{
			Value: uint64(s.Value),
			Name:  s.Name,
		}
		stats = append(stats, stat)
	}

	t.envoyStats = stats
	return stats, resultErr
}
