package client

import "github.com/hashicorp/consul/agent/hcp/telemetry"

type MockMetricsClient struct {
	telemetry.MetricsClient
}
