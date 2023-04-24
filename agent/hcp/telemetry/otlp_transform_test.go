package telemetry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	mpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	rpb "go.opentelemetry.io/proto/otlp/resource/v1"
)

var (

	// Common attributes for test cases.
	start = time.Date(2000, time.January, 01, 0, 0, 0, 0, time.FixedZone("GMT", 0))
	end   = start.Add(30 * time.Second)

	alice = attribute.NewSet(attribute.String("user", "alice"))
	bob   = attribute.NewSet(attribute.String("user", "bob"))

	pbAlice = &cpb.KeyValue{Key: "user", Value: &cpb.AnyValue{
		Value: &cpb.AnyValue_StringValue{StringValue: "alice"},
	}}
	pbBob = &cpb.KeyValue{Key: "user", Value: &cpb.AnyValue{
		Value: &cpb.AnyValue_StringValue{StringValue: "bob"},
	}}

	// DataPoint test case : Histogram Datapoints (Histogram)
	minA, maxA, sumA = 2.0, 4.0, 90.0
	minB, maxB, sumB = 4.0, 150.0, 234.0
	otelHDP          = []metricdata.HistogramDataPoint[float64]{{
		Attributes:   alice,
		StartTime:    start,
		Time:         end,
		Count:        30,
		Bounds:       []float64{1, 5},
		BucketCounts: []uint64{0, 30, 0},
		Min:          metricdata.NewExtrema(minA),
		Max:          metricdata.NewExtrema(maxA),
		Sum:          sumA,
	}, {
		Attributes:   bob,
		StartTime:    start,
		Time:         end,
		Count:        3,
		Bounds:       []float64{1, 5},
		BucketCounts: []uint64{0, 1, 2},
		Min:          metricdata.NewExtrema(minB),
		Max:          metricdata.NewExtrema(maxB),
		Sum:          sumB,
	}}

	otlpHDP = []*mpb.HistogramDataPoint{{
		Attributes:        []*cpb.KeyValue{pbAlice},
		StartTimeUnixNano: uint64(start.UnixNano()),
		TimeUnixNano:      uint64(end.UnixNano()),
		Count:             30,
		Sum:               &sumA,
		ExplicitBounds:    []float64{1, 5},
		BucketCounts:      []uint64{0, 30, 0},
		Min:               &minA,
		Max:               &maxA,
	}, {
		Attributes:        []*cpb.KeyValue{pbBob},
		StartTimeUnixNano: uint64(start.UnixNano()),
		TimeUnixNano:      uint64(end.UnixNano()),
		Count:             3,
		Sum:               &sumB,
		ExplicitBounds:    []float64{1, 5},
		BucketCounts:      []uint64{0, 1, 2},
		Min:               &minB,
		Max:               &maxB,
	}}
	// DataPoint test case : Number Datapoints (Gauge / Counter)
	otelDP = []metricdata.DataPoint[float64]{
		{Attributes: alice, StartTime: start, Time: end, Value: 1.0},
		{Attributes: bob, StartTime: start, Time: end, Value: 2.0},
	}

	otlpDP = []*mpb.NumberDataPoint{
		{
			Attributes:        []*cpb.KeyValue{pbAlice},
			StartTimeUnixNano: uint64(start.UnixNano()),
			TimeUnixNano:      uint64(end.UnixNano()),
			Value:             &mpb.NumberDataPoint_AsDouble{AsDouble: 1.0},
		},
		{
			Attributes:        []*cpb.KeyValue{pbBob},
			StartTimeUnixNano: uint64(start.UnixNano()),
			TimeUnixNano:      uint64(end.UnixNano()),
			Value:             &mpb.NumberDataPoint_AsDouble{AsDouble: 2.0},
		},
	}

	// Metrics Test Case
	// - 3 invalid metrics and 3 Valid to test filtering
	// 		- 1 invalid metric type
	//		- 2 invalid cummulative temporalities (only cummulative supported)
	// - 3 types (Gauge, Counter, and Histogram) supported
	otelMetrics = []metricdata.Metrics{
		{
			Name:        "float64-gauge",
			Description: "Gauge with float64 values",
			Unit:        "1",
			Data:        metricdata.Gauge[float64]{DataPoints: otelDP},
		},
		{
			Name:        "float64-sum",
			Description: "Sum with float64 values",
			Unit:        "1",
			Data: metricdata.Sum[float64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: false,
				DataPoints:  otelDP,
			},
		},
		{
			Name:        "float64-histogram",
			Description: "Histogram",
			Unit:        "1",
			Data: metricdata.Histogram[float64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints:  otelHDP,
			},
		},
		{
			Name:        "invalid-sum",
			Description: "Sum with invalid temporality",
			Unit:        "1",
			Data: metricdata.Sum[float64]{
				Temporality: metricdata.DeltaTemporality,
				IsMonotonic: false,
				DataPoints:  otelDP,
			},
		},
		{
			Name:        "invalid-histogram",
			Description: "Invalid histogram",
			Unit:        "1",
			Data: metricdata.Histogram[float64]{
				Temporality: metricdata.DeltaTemporality,
				DataPoints:  otelHDP,
			},
		},
		{
			Name:        "unknown",
			Description: "Unknown aggregation",
			Unit:        "1",
			Data:        metricdata.Histogram[int64]{},
		},
	}

	otlpMetrics = []*mpb.Metric{
		{
			Name:        "float64-gauge",
			Description: "Gauge with float64 values",
			Unit:        "1",
			Data:        &mpb.Metric_Gauge{Gauge: &mpb.Gauge{DataPoints: otlpDP}},
		},
		{
			Name:        "float64-sum",
			Description: "Sum with float64 values",
			Unit:        "1",
			Data: &mpb.Metric_Sum{Sum: &mpb.Sum{
				AggregationTemporality: mpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				IsMonotonic:            false,
				DataPoints:             otlpDP,
			}},
		},
		{
			Name:        "float64-histogram",
			Description: "Histogram",
			Unit:        "1",
			Data: &mpb.Metric_Histogram{Histogram: &mpb.Histogram{
				AggregationTemporality: mpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints:             otlpHDP,
			}},
		},
	}

	// ScopeMetrics Test Cases
	otelScopeMetrics = []metricdata.ScopeMetrics{{
		Scope: instrumentation.Scope{
			Name:    "test/code/path",
			Version: "v0.1.0",
		},
		Metrics: otelMetrics,
	}}

	otlpScopeMetrics = []*mpb.ScopeMetrics{{
		Scope: &cpb.InstrumentationScope{
			Name:    "test/code/path",
			Version: "v0.1.0",
		},
		Metrics: otlpMetrics,
	}}

	// ResourceMetrics Test Cases
	otelResourceMetrics = &metricdata.ResourceMetrics{
		Resource: resource.NewSchemaless(
			semconv.ServiceName("test server"),
			semconv.ServiceVersion("v0.1.0"),
		),
		ScopeMetrics: otelScopeMetrics,
	}

	otlpResourceMetrics = &mpb.ResourceMetrics{
		Resource: &rpb.Resource{
			Attributes: []*cpb.KeyValue{
				{
					Key: "service.name",
					Value: &cpb.AnyValue{
						Value: &cpb.AnyValue_StringValue{StringValue: "test server"},
					},
				},
				{
					Key: "service.version",
					Value: &cpb.AnyValue{
						Value: &cpb.AnyValue_StringValue{StringValue: "v0.1.0"},
					},
				},
			},
		},
		ScopeMetrics: otlpScopeMetrics,
	}
)

// TestTransformOTLP runs tests from the "bottom-up" of the metricdata data types.
func TestTransformOTLP(t *testing.T) {
	// Histogram DataPoint Test Case (Histograms)
	assert.Equal(t, otlpHDP, histogramDataPoints(otelHDP))

	// Number DataPoint Test Case (Counters / Gauges)
	require.Equal(t, otlpDP, dataPoints(otelDP))

	// Metrics Test Case
	m, err := metrics(otelMetrics)
	require.Equal(t, otlpMetrics, m)
	require.Equal(t, len(otlpMetrics), 3)
	require.Contains(t, err.Error(), "3 errors occurred")

	// Scope Metrics Test Case
	sm, err := scopeMetrics(otelScopeMetrics)
	require.Equal(t, otlpScopeMetrics, sm)
	require.Contains(t, err.Error(), "3 errors occurred")

	// Resource Metrics Test Case
	rm, err := transformOTLP(otelResourceMetrics)
	require.Equal(t, otlpResourceMetrics, rm)
	require.Contains(t, err.Error(), "3 errors occurred")
}
