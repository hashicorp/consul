package hcp

import (
	"testing"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/go-hclog"
)

func TestInitTelemetry(t *testing.T) {
	cfg := config.CloudConfig{}
	logger := hclog.NewNullLogger()
	mClient := client.NewMockClient()

	initTelemetry(mClient, logger, cfg)

}
