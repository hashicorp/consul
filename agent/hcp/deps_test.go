package hcp

import (
	"context"
	"net/url"
	"regexp"
	"testing"
	"time"

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

	u, _ := url.Parse("https://test.com/v1/metrics")
	filters, _ := regexp.Compile("test")

	c := client.NewMockClient(t)
	mt := mockTelemetryConfig(1*time.Second, u, filters)
	c.EXPECT().FetchTelemetryConfig(mock.Anything).Return(mt, nil)

	mc := mockMetricsClient{}
	ctx := context.Background()
	s, err := sink(ctx, c, mc)

	require.NotNil(t, s)
	require.NoError(t, err)
}

func mockTelemetryConfig(refreshInterval time.Duration, metricsEndpoint *url.URL, filters *regexp.Regexp) *client.TelemetryConfig {
	return &client.TelemetryConfig{
		MetricsConfig: &client.MetricsConfig{
			Endpoint: metricsEndpoint,
			Filters:  filters,
		},
		RefreshConfig: &client.RefreshConfig{
			RefreshInterval: refreshInterval,
		},
	}
}
