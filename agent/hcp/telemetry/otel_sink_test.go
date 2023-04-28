package telemetry

import (
	"context"
	"io"
	"testing"

	gometrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	attrs = attribute.NewSet(attribute.KeyValue{
		Key:   attribute.Key("server.id"),
		Value: attribute.StringValue("test"),
	})

	expectedMetrics = map[string]metricdata.Metrics{
		"consul.raft.leader": {
			Name:        "consul.raft.leader",
			Description: "",
			Unit:        "",
			Data: metricdata.Gauge[float64]{
				DataPoints: []metricdata.DataPoint[float64]{
					{
						Attributes: *attribute.EmptySet(),
						Value:      float64(float32(0)),
					},
				},
			},
		},
		"consul.autopilot.healthy": {
			Name:        "consul.autopilot.healthy",
			Description: "",
			Unit:        "",
			Data: metricdata.Gauge[float64]{
				DataPoints: []metricdata.DataPoint[float64]{
					{
						Attributes: attrs,
						Value:      float64(float32(1.23)),
					},
				},
			},
		},
		"consul.raft.state.leader": {
			Name:        "consul.raft.state.leader",
			Description: "",
			Unit:        "",
			Data: metricdata.Sum[float64]{
				DataPoints: []metricdata.DataPoint[float64]{
					{
						Attributes: *attribute.EmptySet(),
						Value:      float64(float32(23.23)),
					},
				},
			},
		},
		"consul.raft.apply": {
			Name:        "consul.raft.apply",
			Description: "",
			Unit:        "",
			Data: metricdata.Sum[float64]{
				DataPoints: []metricdata.DataPoint[float64]{
					{
						Attributes: attrs,
						Value:      float64(float32(1.44)),
					},
				},
			},
		},
		"consul.raft.leader.lastContact": {
			Name:        "consul.raft.leader.lastContact",
			Description: "",
			Unit:        "",
			Data: metricdata.Histogram[float64]{
				DataPoints: []metricdata.HistogramDataPoint[float64]{
					{
						Attributes: *attribute.EmptySet(),
						Count:      1,
						Sum:        float64(float32(45.32)),
						Min:        metricdata.NewExtrema(float64(float32(45.32))),
						Max:        metricdata.NewExtrema(float64(float32(45.32))),
					},
				},
			},
		},
		"consul.raft.commitTime": {
			Name:        "consul.raft.commitTime",
			Description: "",
			Unit:        "",
			Data: metricdata.Histogram[float64]{
				DataPoints: []metricdata.HistogramDataPoint[float64]{
					{
						Attributes: attrs,
						Count:      1,
						Sum:        float64(float32(26.34)),
						Min:        metricdata.NewExtrema(float64(float32(26.34))),
						Max:        metricdata.NewExtrema(float64(float32(26.34))),
					},
				},
			},
		},
	}
)

func TestNewOTELSink(t *testing.T) {
	for name, test := range map[string]struct {
		wantErr string
		opts    *OTELSinkOpts
	}{
		"failsWithEmptyLogger": {
			wantErr: "failed to init OTEL sink: provide valid OTELSinkOpts Logger",
			opts: &OTELSinkOpts{
				Logger: nil,
				Reader: metric.NewManualReader(),
			},
		},
		"failsWithEmptyReader": {
			wantErr: "failed to init OTEL sink: provide valid OTELSinkOpts Reader",
			opts: &OTELSinkOpts{
				Logger: hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
				Reader: nil,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			sink, err := NewOTELSink(test.opts)
			if test.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.wantErr)
				return
			}

			require.NotNil(t, sink)
		})
	}
}

func TestOTELSink(t *testing.T) {
	// Manual reader outputs the aggregated metrics when reader.Collect is called.
	reader := metric.NewManualReader()

	ctx := context.Background()
	opts := &OTELSinkOpts{
		Logger: hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		Reader: reader,
	}

	sink, err := NewOTELSink(opts)
	require.NoError(t, err)

	labels := []gometrics.Label{
		{
			Name:  "server.id",
			Value: "test",
		},
	}

	sink.SetGauge([]string{"consul", "raft", "leader"}, float32(0))
	sink.SetGaugeWithLabels([]string{"consul", "autopilot", "healthy"}, float32(1.23), labels)

	sink.IncrCounter([]string{"consul", "raft", "state", "leader"}, float32(23.23))
	sink.IncrCounterWithLabels([]string{"consul", "raft", "apply"}, float32(1.44), labels)

	sink.AddSample([]string{"consul", "raft", "leader", "lastContact"}, float32(45.32))
	sink.AddSampleWithLabels([]string{"consul", "raft", "commitTime"}, float32(26.34), labels)

	var collected metricdata.ResourceMetrics
	err = reader.Collect(ctx, &collected)
	require.NoError(t, err)

	// Validate resource
	require.Equal(t, resource.NewSchemaless(), collected.Resource)

	// Validate metrics
	for _, actual := range collected.ScopeMetrics[0].Metrics {
		name := actual.Name
		expected, ok := expectedMetrics[name]
		require.True(t, ok, "metric key %s should be in expectedMetrics map", name)
		isSameMetrics(t, expected, actual)
	}
}

// compareMetrics verifies if two metricdata.Metric objects are equal by ignoring the time component.
func isSameMetrics(t *testing.T, expected metricdata.Metrics, actual metricdata.Metrics) {
	require.Equal(t, expected.Name, actual.Name, "different .Name field")
	require.Equal(t, expected.Description, actual.Description, "different .Description field")
	require.Equal(t, expected.Unit, actual.Unit, "different .Unit field")

	switch expectedData := expected.Data.(type) {
	case metricdata.Gauge[float64]:
		actualData, ok := actual.Data.(metricdata.Gauge[float64])
		require.True(t, ok, "different metric types: expected metricdata.Gauge[float64]")

		isSameData(t, expectedData.DataPoints, actualData.DataPoints)
	case metricdata.Sum[float64]:
		actualData, ok := actual.Data.(metricdata.Sum[float64])
		require.True(t, ok, "different metric types: expected metricdata.Sum[float64]")

		isSameData(t, expectedData.DataPoints, actualData.DataPoints)
	case metricdata.Histogram[float64]:
		actualData, ok := actual.Data.(metricdata.Histogram[float64])
		require.True(t, ok, "different metric types: expected metricdata.Histogram[float64]")

		isSameHistogramData(t, expectedData.DataPoints, actualData.DataPoints)
	}
}

func isSameData(t *testing.T, expected []metricdata.DataPoint[float64], actual []metricdata.DataPoint[float64]) {
	require.Equal(t, len(expected), len(actual), "different datapoints length")

	// Only verify the value and the attributes.
	for i, dp := range expected {
		currActual := actual[i]
		require.Equal(t, dp.Value, currActual.Value, "different datapoint value")
		require.Equal(t, dp.Attributes.Len(), currActual.Attributes.Len(), "different attributes of datapoint length")

		iter := dp.Attributes.Iter()
		for iter.Next() {
			attr := iter.Attribute()
			require.True(t, currActual.Attributes.HasValue(attr.Key), "missing attribute in expected")
		}
	}
}

func isSameHistogramData(t *testing.T, expected []metricdata.HistogramDataPoint[float64], actual []metricdata.HistogramDataPoint[float64]) {
	require.Equal(t, len(expected), len(actual), "different histogram datapoint length")

	// Only verify the value and the attributes.
	for i, dp := range expected {
		currActual := actual[i]
		require.Equal(t, dp.Sum, currActual.Sum, "different histogram datapoint .Sum value")
		require.Equal(t, dp.Max, currActual.Max, "different histogram datapoint .Max value")
		require.Equal(t, dp.Min, currActual.Min, "different histogram datapoint .Min value")
		require.Equal(t, dp.Count, currActual.Count, "different histogram datapoint .Count value")

		require.Equal(t, dp.Attributes.Len(), currActual.Attributes.Len(), "different attributes of datapoint length")

		iter := dp.Attributes.Iter()
		for iter.Next() {
			attr := iter.Attribute()
			require.True(t, currActual.Attributes.HasValue(attr.Key), "missing attribute in expected")
		}
	}
}
