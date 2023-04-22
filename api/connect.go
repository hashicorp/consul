// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

// HCPMetricsCollectorName is the service name for the HCP Metrics Collector
const HCPMetricsCollectorName string = "hcp-metrics-collector"

// Connect can be used to work with endpoints related to Connect, the
// feature for securely connecting services within Consul.
type Connect struct {
	c *Client
}

// Connect returns a handle to the connect-related endpoints
func (c *Client) Connect() *Connect {
	return &Connect{c}
}
