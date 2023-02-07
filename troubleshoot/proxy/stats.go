package troubleshoot

import (
	"encoding/json"
	"fmt"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
)

type statsJson struct {
	Stats []simpleMetric `json:"stats"`
}

type simpleMetric struct {
	Value int64  `json:"value,omitempty"`
	Name  string `json:"name,omitempty"`
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
