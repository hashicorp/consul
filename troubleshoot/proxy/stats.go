// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package troubleshoot

import (
	"encoding/json"
	"fmt"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	"github.com/hashicorp/consul/troubleshoot/validate"
)

type statsJson struct {
	Stats []simpleMetric `json:"stats"`
}

type simpleMetric struct {
	Value int64  `json:"value,omitempty"`
	Name  string `json:"name,omitempty"`
}

func (t *Troubleshoot) troubleshootStats() (validate.Messages, error) {

	statMessages := validate.Messages{}

	rejectionStats, err := t.getEnvoyStats("update_rejected")
	if err != nil {
		return nil, fmt.Errorf("could not get config rejection stats from envoy admin API: %w", err)
	}

	totalConfigRejections := 0
	for _, rs := range rejectionStats {
		if rs.Value != 0 {
			totalConfigRejections += int(rs.Value)
		}
	}

	if totalConfigRejections > 0 {
		statMessages = append(statMessages, validate.Message{
			Message: fmt.Sprintf("Envoy has %v rejected configurations", totalConfigRejections),
			PossibleActions: []string{
				"Check the logs of the Consul agent configuring the local proxy to see why Envoy rejected this configuration",
			},
		})
	} else {
		statMessages = append(statMessages, validate.Message{
			Success: true,
			Message: fmt.Sprintf("Envoy has %v rejected configurations", totalConfigRejections),
		})
	}

	connectionFailureStats, err := t.getEnvoyStats("upstream_cx_connect_fail")
	if err != nil {
		return nil, fmt.Errorf("could not get connection failure stats from envoy admin API: %w", err)
	}

	totalCxFailures := 0
	for _, cfs := range connectionFailureStats {
		if cfs.Value != 0 {
			totalCxFailures += int(cfs.Value)
		}
	}
	statMessages = append(statMessages, validate.Message{
		Success: true,
		Message: fmt.Sprintf("Envoy has detected %v connection failure(s)", totalCxFailures),
	})
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
