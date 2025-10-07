// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package agent

import "github.com/hashicorp/go-metrics"

func (s *HTTPHandlers) nodeMetricsLabels() []metrics.Label {
	return []metrics.Label{{Name: "node", Value: s.nodeName()}}
}
