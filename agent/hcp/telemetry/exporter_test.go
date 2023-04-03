package telemetry

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/go-hclog"
)

func TestExporter_NewMetricsExporter(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg         *MetricsExporterConfig
		wantErr     string
		expectedExp *MetricsExporter
	}{
		"failsWithoutClient": {
			cfg: &MetricsExporterConfig{
				Client:  nil,
				Filters: []string{"raft.apply$"},
				Logger:  hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
			},
			wantErr: "HCP client and a logger are required",
		},
		"failsWithoutLogger": {
			cfg: &MetricsExporterConfig{
				Client:  nil,
				Filters: []string{"raft.apply$"},
			},
			wantErr: "HCP client and a logger are required",
		},
		"invalidFilter": {
			cfg: &MetricsExporterConfig{
				Client: hcpclient.NewMockClient(t),
				// Unsupported re2 regex syntax
				Filters: []string{"(*LF)"},
				Logger:  hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
			},
			wantErr: "invalid regex",
		},
		"success": {
			cfg: &MetricsExporterConfig{
				Client:  hcpclient.NewMockClient(t),
				Filters: []string{"raft.apply$"},
				Labels: map[string]string{
					"instance.id": "asdkfj",
				},
				Logger: hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			exp, err := NewMetricsExporter(tc.cfg)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				require.Nil(t, exp)
			} else {
				for _, v := range tc.cfg.Filters {
					_, ok := exp.filter.filters[v]
					require.True(t, ok)
				}
				attr := make([]attribute.KeyValue, 0, len(tc.cfg.Labels))
				for k, v := range tc.cfg.Labels {
					attr = append(attr, attribute.KeyValue{
						Key:   attribute.Key(k),
						Value: attribute.StringValue(v),
					})
				}
				require.Equal(t, exp.resource, resource.NewWithAttributes("", attr...))
				require.NotNil(t, exp.logger)
				require.NotNil(t, exp.client)
			}
		})
	}
}

func TestExporter_Export(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Second)

	for name, test := range map[string]struct {
		goMetrics    []*metrics.IntervalMetrics
		expectedOTLP *metricdata.ResourceMetrics
		filters      []string
		labels       map[string]string
	}{
		"success": {
			filters: []string{"raft.*"},
			goMetrics: []*metrics.IntervalMetrics{
				{
					Interval: now,
					Gauges: map[string]metrics.GaugeValue{
						"consul.raft.peers": {
							Name:  "consul.raft.peers",
							Value: 2.0,
							Labels: []metrics.Label{
								{
									Name:  "peer_type",
									Value: "test",
								},
							},
						},
					},
					Counters: map[string]metrics.SampledValue{
						"consul.raft.fsm.apply": {
							Name: "consul.raft.fsm.apply",
							AggregateSample: &metrics.AggregateSample{
								Sum: 4.0,
							},
						},
					},
					Samples: map[string]metrics.SampledValue{
						"consul.raft.fsm.enqueue": {
							Name: "consul.raft.fsm.enqueue",
							AggregateSample: &metrics.AggregateSample{
								Sum:   8.0,
								Min:   1.0,
								Max:   14.0,
								Count: 10,
							},
						},
					},
				},
				{
					Interval: later,
					Gauges: map[string]metrics.GaugeValue{
						"consul.raft.last_index": {
							Name:  "consul.raft.last_index",
							Value: 1.0,
						},
					},
				},
			},
			expectedOTLP: &metricdata.ResourceMetrics{
				Resource: resource.NewWithAttributes(""),
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Scope: instrumentation.Scope{
							Name:    "github.com/hashicorp/consul/agent/hcp/client/telemetry",
							Version: "v1",
						},
						Metrics: []metricdata.Metrics{
							{
								Name: "consul.raft.peers",
								Data: metricdata.Gauge[float64]{
									DataPoints: []metricdata.DataPoint[float64]{
										{
											Time:  now,
											Value: 2.0,
											Attributes: attribute.NewSet(
												attribute.KeyValue{
													Key:   attribute.Key("peer_type"),
													Value: attribute.StringValue("test"),
												},
											),
										},
									},
								},
							},
							{
								Name: "consul.raft.fsm.apply",
								Data: metricdata.Sum[float64]{
									Temporality: metric.DefaultTemporalitySelector(metric.InstrumentKindCounter),
									DataPoints: []metricdata.DataPoint[float64]{
										{
											Attributes: *attribute.EmptySet(),
											Time:       now,
											Value:      4.0,
										},
									},
								},
							},
							{
								Name: "consul.raft.fsm.enqueue",
								Data: metricdata.Histogram{
									Temporality: metric.DefaultTemporalitySelector(metric.InstrumentKindHistogram),
									DataPoints: []metricdata.HistogramDataPoint{
										{
											Attributes: *attribute.EmptySet(),
											Sum:        8.0,
											Min:        metricdata.NewExtrema(1.0),
											Max:        metricdata.NewExtrema(14.0),
											Time:       now,
											Count:      uint64(10),
										},
									},
								},
							},
							{
								Name: "consul.raft.last_index",
								Data: metricdata.Gauge[float64]{
									DataPoints: []metricdata.DataPoint[float64]{
										{
											Attributes: *attribute.EmptySet(),
											Time:       later,
											Value:      1.0,
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"withFilteredMetrics": {
			filters: []string{"raft.*"},
			goMetrics: []*metrics.IntervalMetrics{
				{
					Interval: now,
					Gauges: map[string]metrics.GaugeValue{
						"consul.raft.peers": {
							Name:  "consul.raft.peers",
							Value: 2.0,
							Labels: []metrics.Label{
								{
									Name:  "peer_type",
									Value: "test",
								},
							},
						},
						"filtered_gauges": {
							Name:  "filtered_gauges",
							Value: 2.0,
						},
					},
					Counters: map[string]metrics.SampledValue{
						"filtered_counters": {
							Name: "filtered_counters",
							AggregateSample: &metrics.AggregateSample{
								Sum: 5.0,
							},
						},
					},
					Samples: map[string]metrics.SampledValue{
						"filtered_samples": {
							Name: "filtered_samples",
							AggregateSample: &metrics.AggregateSample{
								Sum:   2.0,
								Min:   1.0,
								Max:   3.0,
								Count: 2,
							},
						},
					},
				},
			},
			expectedOTLP: &metricdata.ResourceMetrics{
				Resource: resource.NewWithAttributes(""),
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Scope: instrumentation.Scope{
							Name:    "github.com/hashicorp/consul/agent/hcp/client/telemetry",
							Version: "v1",
						},
						Metrics: []metricdata.Metrics{
							{
								Name: "consul.raft.peers",
								Data: metricdata.Gauge[float64]{
									DataPoints: []metricdata.DataPoint[float64]{
										{
											Time:  now,
											Value: 2.0,
											Attributes: attribute.NewSet(
												attribute.KeyValue{
													Key:   attribute.Key("peer_type"),
													Value: attribute.StringValue("test"),
												},
											),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"withLabels": {
			filters: []string{"raft.*"},
			labels: map[string]string{
				"instance.id": "testserver",
			},
			goMetrics: []*metrics.IntervalMetrics{
				{
					Interval: now,
					Gauges: map[string]metrics.GaugeValue{
						"consul.raft.peers": {
							Name:  "consul.raft.peers",
							Value: 2.0,
							Labels: []metrics.Label{
								{
									Name:  "peer_type",
									Value: "test",
								},
							},
						},
					},
				},
			},
			expectedOTLP: &metricdata.ResourceMetrics{
				Resource: resource.NewWithAttributes("", attribute.KeyValue{
					Key:   attribute.Key("instance.id"),
					Value: attribute.StringValue("testserver"),
				}),
				ScopeMetrics: []metricdata.ScopeMetrics{
					{
						Scope: instrumentation.Scope{
							Name:    "github.com/hashicorp/consul/agent/hcp/client/telemetry",
							Version: "v1",
						},
						Metrics: []metricdata.Metrics{
							{
								Name: "consul.raft.peers",
								Data: metricdata.Gauge[float64]{
									DataPoints: []metricdata.DataPoint[float64]{
										{
											Time:  now,
											Value: 2.0,
											Attributes: attribute.NewSet(
												attribute.KeyValue{
													Key:   attribute.Key("peer_type"),
													Value: attribute.StringValue("test"),
												},
											),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"emptyGoMetrics": {
			goMetrics:    []*metrics.IntervalMetrics{},
			expectedOTLP: nil,
		},
		"emptyOTLPMetricsWithoutFilters": {
			goMetrics: []*metrics.IntervalMetrics{
				{
					Interval: now,
					Gauges: map[string]metrics.GaugeValue{
						"consul.raft.peers": {
							Name:  "consul.raft.peers",
							Value: 2.0,
							Labels: []metrics.Label{
								{
									Name:  "peer_type",
									Value: "test",
								},
							},
						},
					},
				},
			},
			expectedOTLP: nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			client := hcpclient.NewMockClient(t)
			cfg := &MetricsExporterConfig{
				Client:  client,
				Filters: test.filters,
				Labels:  test.labels,
				Logger:  hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
			}
			exp, err := NewMetricsExporter(cfg)
			require.NoError(t, err)

			if test.expectedOTLP != nil {
				client.EXPECT().ExportMetrics(ctx, test.expectedOTLP).Once().Return(nil)
			}

			err = exp.Export(ctx, test.goMetrics)
			require.NoError(t, err)
		})
	}
}
