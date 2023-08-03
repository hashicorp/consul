package hcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/telemetry"
)

type mockMetricsClient struct {
	telemetry.MetricsClient
}

func TestSink(t *testing.T) {
	t.Parallel()

	// FetchTelemetryConfig is stubbed for TelemetryConfigProvider,which periodically fetches metrics configuration from HCP.
	c := &client.MockClient{}
	c.EXPECT().FetchTelemetryConfig(mock.Anything).Return(nil, errors.New("fetch error: using default provider settings"))

	mc := mockMetricsClient{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := sink(ctx, c, mc)

	require.NotNil(t, s)
	require.NoError(t, err)
}
