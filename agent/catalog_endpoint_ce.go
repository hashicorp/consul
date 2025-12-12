// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package agent

import "github.com/armon/go-metrics"

func (s *HTTPHandlers) nodeMetricsLabels() []metrics.Label {
	return []metrics.Label{{Name: "node", Value: s.nodeName()}}
}
