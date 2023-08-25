package telemetry

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	gometrics "github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

type mockConfigProvider struct {
	filter *regexp.Regexp
	labels map[string]string
}

func (m *mockConfigProvider) GetLabels() map[string]string {
	return m.labels
}

func (m *mockConfigProvider) GetFilters() *regexp.Regexp {
	return m.filter
}

var (
	expectedResource = resource.NewSchemaless()

	attrs = attribute.NewSet(attribute.KeyValue{
		Key:   attribute.Key("node_id"),
		Value: attribute.StringValue("test"),
	})
	attrsWithMetricLabel = attribute.NewSet(attribute.KeyValue{
		Key:   attribute.Key("metric.label"),
		Value: attribute.StringValue("test"),
	}, attribute.KeyValue{
		Key:   attribute.Key("node_id"),
		Value: attribute.StringValue("test"),
	})

	expectedSinkMetrics = map[string]metricdata.Metrics{
		"consul.raft.leader": {
			Name:        "consul.raft.leader",
			Description: "",
			Unit:        "",
			Data: metricdata.Gauge[float64]{
				DataPoints: []metricdata.DataPoint[float64]{
					{
						Attributes: attrs,
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
						Attributes: attrsWithMetricLabel,
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
						Attributes: attrs,
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
						Attributes: attrsWithMetricLabel,
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
						Attributes: attrs,
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
						Attributes: attrsWithMetricLabel,
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
	t.Parallel()
	for name, test := range map[string]struct {
		wantErr string
		opts    *OTELSinkOpts
	}{
		"failsWithEmptyReader": {
			wantErr: "ferror: provide valid reader",
			opts: &OTELSinkOpts{
				Reader:         nil,
				ConfigProvider: &mockConfigProvider{},
			},
		},
		"failsWithEmptyConfigProvider": {
			wantErr: "ferror: provide valid config provider",
			opts: &OTELSinkOpts{
				Reader: metric.NewManualReader(),
			},
		},
		"success": {
			opts: &OTELSinkOpts{
				Reader:         metric.NewManualReader(),
				ConfigProvider: &mockConfigProvider{},
			},
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			sink, err := NewOTELSink(context.Background(), test.opts)
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
	t.Parallel()

	// Manual reader outputs the aggregated metrics when reader.Collect is called.
	reader := metric.NewManualReader()

	ctx := context.Background()
	opts := &OTELSinkOpts{
		Reader: reader,
		ConfigProvider: &mockConfigProvider{
			filter: regexp.MustCompile("raft|autopilot"),
			labels: map[string]string{
				"node_id": "test",
			},
		},
	}

	sink, err := NewOTELSink(ctx, opts)
	require.NoError(t, err)

	labels := []gometrics.Label{
		{
			Name:  "metric.label",
			Value: "test",
		},
	}

	sink.SetGauge([]string{"test", "bad_filter", "gauge"}, float32(0))
	sink.SetGauge([]string{"consul", "raft", "leader"}, float32(0))
	sink.SetGaugeWithLabels([]string{"consul", "autopilot", "healthy"}, float32(1.23), labels)

	sink.IncrCounter([]string{"test", "bad_filter", "counter"}, float32(23.23))
	sink.IncrCounter([]string{"consul", "raft", "state", "leader"}, float32(23.23))
	sink.IncrCounterWithLabels([]string{"consul", "raft", "apply"}, float32(1.44), labels)

	sink.AddSample([]string{"test", "bad_filter", "sample"}, float32(45.32))
	sink.AddSample([]string{"consul", "raft", "leader", "lastContact"}, float32(45.32))
	sink.AddSampleWithLabels([]string{"consul", "raft", "commitTime"}, float32(26.34), labels)

	var collected metricdata.ResourceMetrics
	err = reader.Collect(ctx, &collected)
	require.NoError(t, err)

	isSame(t, expectedSinkMetrics, collected)
}

func TestLabelsToAttributes(t *testing.T) {
	for name, test := range map[string]struct {
		providerLabels         map[string]string
		goMetricsLabels        []gometrics.Label
		expectedOTELAttributes []attribute.KeyValue
	}{
		"emptyLabels": {
			expectedOTELAttributes: []attribute.KeyValue{},
		},
		"emptyGoMetricsLabels": {
			providerLabels: map[string]string{
				"node_id": "test",
			},
			expectedOTELAttributes: []attribute.KeyValue{
				{
					Key:   attribute.Key("node_id"),
					Value: attribute.StringValue("test"),
				},
			},
		},
		"emptyProviderLabels": {
			goMetricsLabels: []gometrics.Label{
				{
					Name:  "server_type",
					Value: "internal",
				},
			},
			expectedOTELAttributes: []attribute.KeyValue{
				{
					Key:   attribute.Key("server_type"),
					Value: attribute.StringValue("internal"),
				},
			},
		},
		"combinedLabels": {
			goMetricsLabels: []gometrics.Label{
				{
					Name:  "server_type",
					Value: "internal",
				},
				{
					Name:  "method",
					Value: "get",
				},
			},
			providerLabels: map[string]string{
				"node_id":   "test",
				"node_name": "labels_test",
			},
			expectedOTELAttributes: []attribute.KeyValue{
				{
					Key:   attribute.Key("server_type"),
					Value: attribute.StringValue("internal"),
				},
				{
					Key:   attribute.Key("method"),
					Value: attribute.StringValue("get"),
				},
				{
					Key:   attribute.Key("node_id"),
					Value: attribute.StringValue("test"),
				},
				{
					Key:   attribute.Key("node_name"),
					Value: attribute.StringValue("labels_test"),
				},
			},
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			opts := &OTELSinkOpts{
				Reader: metric.NewManualReader(),
				ConfigProvider: &mockConfigProvider{
					filter: regexp.MustCompile("raft|autopilot"),
					labels: test.providerLabels,
				},
			}
			sink, err := NewOTELSink(ctx, opts)
			require.NoError(t, err)

			require.Equal(t, test.expectedOTELAttributes, sink.labelsToAttributes(test.goMetricsLabels))
		})
	}
}

func TestOTELSinkFilters(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		cfgProvider ConfigProvider
		expected    bool
	}{
		"emptyMatch": {
			cfgProvider: &mockConfigProvider{},
			expected:    true,
		},
		"matchingFilter": {
			cfgProvider: &mockConfigProvider{
				filter: regexp.MustCompile("raft"),
			},
			expected: true,
		},
		"mismatchFilter": {cfgProvider: &mockConfigProvider{
			filter: regexp.MustCompile("test"),
		}},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testMetricKey := "consul.raft"
			s, err := NewOTELSink(context.Background(), &OTELSinkOpts{
				ConfigProvider: tc.cfgProvider,
				Reader:         metric.NewManualReader(),
			})
			require.NoError(t, err)
			require.Equal(t, tc.expected, s.allowedMetric(testMetricKey))
		})
	}
}

func TestOTELSink_Race(t *testing.T) {
	reader := metric.NewManualReader()
	ctx := context.Background()
	defaultLabels := map[string]string{
		"node_id": "test",
	}
	opts := &OTELSinkOpts{
		Reader: reader,
		ConfigProvider: &mockConfigProvider{
			filter: regexp.MustCompile("test"),
			labels: defaultLabels,
		},
	}

	sink, err := NewOTELSink(context.Background(), opts)
	require.NoError(t, err)

	samples := 100
	expectedMetrics := generateSamples(samples, defaultLabels)
	wg := &sync.WaitGroup{}
	errCh := make(chan error, samples)
	for k, v := range expectedMetrics {
		wg.Add(1)
		go func(k string, v metricdata.Metrics) {
			defer wg.Done()
			performSinkOperation(sink, k, v, errCh)
		}(k, v)
	}
	wg.Wait()

	require.Empty(t, errCh)

	var collected metricdata.ResourceMetrics
	err = reader.Collect(ctx, &collected)
	require.NoError(t, err)

	isSame(t, expectedMetrics, collected)
}

// generateSamples generates n of each gauges, counter and histogram measurements to use for test purposes.
func generateSamples(n int, labels map[string]string) map[string]metricdata.Metrics {
	generated := make(map[string]metricdata.Metrics, 3*n)
	attrs := *attribute.EmptySet()

	kvs := make([]attribute.KeyValue, 0, len(labels))
	for k, v := range labels {
		kvs = append(kvs, attribute.KeyValue{Key: attribute.Key(k), Value: attribute.StringValue(v)})
	}
	if len(kvs) > 0 {
		attrs = attribute.NewSet(kvs...)
	}

	for i := 0; i < n; i++ {
		v := 12.3
		k := fmt.Sprintf("consul.test.gauges.%d", i)
		generated[k] = metricdata.Metrics{
			Name: k,
			Data: metricdata.Gauge[float64]{
				DataPoints: []metricdata.DataPoint[float64]{
					{
						Attributes: attrs,
						Value:      float64(float32(v)),
					},
				},
			},
		}
	}

	for i := 0; i < n; i++ {
		v := 22.23
		k := fmt.Sprintf("consul.test.sum.%d", i)
		generated[k] = metricdata.Metrics{
			Name: k,
			Data: metricdata.Sum[float64]{
				DataPoints: []metricdata.DataPoint[float64]{
					{
						Attributes: attrs,
						Value:      float64(float32(v)),
					},
				},
			},
		}

	}

	for i := 0; i < n; i++ {
		v := 13.24
		k := fmt.Sprintf("consul.test.hist.%d", i)
		generated[k] = metricdata.Metrics{
			Name: k,
			Data: metricdata.Histogram[float64]{
				DataPoints: []metricdata.HistogramDataPoint[float64]{
					{
						Attributes: attrs,
						Sum:        float64(float32(v)),
						Max:        metricdata.NewExtrema(float64(float32(v))),
						Min:        metricdata.NewExtrema(float64(float32(v))),
						Count:      1,
					},
				},
			},
		}
	}

	return generated
}

// performSinkOperation emits a measurement using the OTELSink and calls wg.Done() when completed.
func performSinkOperation(sink *OTELSink, k string, v metricdata.Metrics, errCh chan error) {
	key := strings.Split(k, ".")
	data := v.Data
	switch data.(type) {
	case metricdata.Gauge[float64]:
		gauge, ok := data.(metricdata.Gauge[float64])
		if !ok {
			errCh <- fmt.Errorf("unexpected type assertion error for key: %s", key)
		}
		sink.SetGauge(key, float32(gauge.DataPoints[0].Value))
	case metricdata.Sum[float64]:
		sum, ok := data.(metricdata.Sum[float64])
		if !ok {
			errCh <- fmt.Errorf("unexpected type assertion error for key: %s", key)
		}
		sink.IncrCounter(key, float32(sum.DataPoints[0].Value))
	case metricdata.Histogram[float64]:
		hist, ok := data.(metricdata.Histogram[float64])
		if !ok {
			errCh <- fmt.Errorf("unexpected type assertion error for key: %s", key)
		}
		sink.AddSample(key, float32(hist.DataPoints[0].Sum))
	}
}

func isSame(t *testing.T, expectedMap map[string]metricdata.Metrics, actual metricdata.ResourceMetrics) {
	// Validate resource
	require.Equal(t, expectedResource, actual.Resource)

	// Validate Metrics
	require.NotEmpty(t, actual.ScopeMetrics)
	actualMetrics := actual.ScopeMetrics[0].Metrics
	require.Equal(t, len(expectedMap), len(actualMetrics))

	for _, actual := range actualMetrics {
		name := actual.Name
		expected, ok := expectedMap[actual.Name]
		require.True(t, ok, "metric key %s should be in expectedMetrics map", name)
		isSameMetrics(t, expected, actual)
	}
}

// compareMetrics verifies if two metricdata.Metric objects are equal by ignoring the time component.
// avoid duplicate datapoint values to ensure predictable order of sort.
func isSameMetrics(t *testing.T, expected metricdata.Metrics, actual metricdata.Metrics) {
	require.Equal(t, expected.Name, actual.Name, "different .Name field")
	require.Equal(t, expected.Description, actual.Description, "different .Description field")
	require.Equal(t, expected.Unit, actual.Unit, "different .Unit field")

	switch expectedData := expected.Data.(type) {
	case metricdata.Gauge[float64]:
		actualData, ok := actual.Data.(metricdata.Gauge[float64])
		require.True(t, ok, "different metric types: expected metricdata.Gauge[float64]")

		isSameDataPoint(t, expectedData.DataPoints, actualData.DataPoints)
	case metricdata.Sum[float64]:
		actualData, ok := actual.Data.(metricdata.Sum[float64])
		require.True(t, ok, "different metric types: expected metricdata.Sum[float64]")

		isSameDataPoint(t, expectedData.DataPoints, actualData.DataPoints)
	case metricdata.Histogram[float64]:
		actualData, ok := actual.Data.(metricdata.Histogram[float64])
		require.True(t, ok, "different metric types: expected metricdata.Histogram")

		isSameHistogramData(t, expectedData.DataPoints, actualData.DataPoints)
	}
}

func isSameDataPoint(t *testing.T, expected []metricdata.DataPoint[float64], actual []metricdata.DataPoint[float64]) {
	require.Equal(t, len(expected), len(actual), "different datapoints length")

	// Sort for predictable data in order of lowest value.
	sort.Slice(expected, func(i, j int) bool {
		return expected[i].Value < expected[j].Value
	})
	sort.Slice(actual, func(i, j int) bool {
		return expected[i].Value < expected[j].Value
	})

	// Only verify the value and  attributes.
	for i, dp := range expected {
		currActual := actual[i]
		require.Equal(t, dp.Value, currActual.Value, "different datapoint value")
		require.Equal(t, dp.Attributes, currActual.Attributes, "different attributes")
	}
}

func isSameHistogramData(t *testing.T, expected []metricdata.HistogramDataPoint[float64], actual []metricdata.HistogramDataPoint[float64]) {
	require.Equal(t, len(expected), len(actual), "different histogram datapoint length")

	// Sort for predictable data in order of lowest sum.
	sort.Slice(expected, func(i, j int) bool {
		return expected[i].Sum < expected[j].Sum
	})
	sort.Slice(actual, func(i, j int) bool {
		return expected[i].Sum < expected[j].Sum
	})

	// Only verify the value and the attributes.
	for i, dp := range expected {
		currActual := actual[i]
		require.Equal(t, dp.Sum, currActual.Sum, "different histogram datapoint .Sum value")
		require.Equal(t, dp.Max, currActual.Max, "different histogram datapoint .Max value")
		require.Equal(t, dp.Min, currActual.Min, "different histogram datapoint .Min value")
		require.Equal(t, dp.Count, currActual.Count, "different histogram datapoint .Count value")
		require.Equal(t, dp.Attributes, currActual.Attributes, "different attributes")
	}
}
