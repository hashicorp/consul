// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package agent

import "github.com/armon/go-metrics"

func (s *HTTPHandlers) nodeMetricsLabels() []metrics.Label {
	return []metrics.Label{{Name: "node", Value: s.nodeName()}}
}
