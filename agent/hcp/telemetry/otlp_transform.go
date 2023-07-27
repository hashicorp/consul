package telemetry

import (
	"errors"
	"fmt"

	goMetrics "github.com/armon/go-metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	mpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	rpb "go.opentelemetry.io/proto/otlp/resource/v1"
)

var (
	errAggregaton  = errors.New("unsupported aggregation")
	errTemporality = errors.New("unsupported temporality")
)

// isEmpty verifies if the given OTLP protobuf metrics contains metric data.
// isEmpty returns true if no ScopeMetrics exist or all metrics within ScopeMetrics are empty.
func isEmpty(rm *mpb.ResourceMetrics) bool {
	// No ScopeMetrics
	if len(rm.ScopeMetrics) == 0 {
		return true
	}

	// If any inner metrics contain data, return false.
	for _, v := range rm.ScopeMetrics {
		if len(v.Metrics) != 0 {
			return false
		}
	}

	// All inner metrics are empty.
	return true
}

// TransformOTLP returns an OTLP ResourceMetrics generated from OTEL metrics. If rm
// contains invalid ScopeMetrics, an error will be returned along with an OTLP
// ResourceMetrics that contains partial OTLP ScopeMetrics.
func transformOTLP(rm *metricdata.ResourceMetrics) *mpb.ResourceMetrics {
	sms := scopeMetricsToPB(rm.ScopeMetrics)
	return &mpb.ResourceMetrics{
		Resource: &rpb.Resource{
			Attributes: attributesToPB(rm.Resource.Iter()),
		},
		ScopeMetrics: sms,
	}
}

// scopeMetrics returns a slice of OTLP ScopeMetrics.
func scopeMetricsToPB(scopeMetrics []metricdata.ScopeMetrics) []*mpb.ScopeMetrics {
	out := make([]*mpb.ScopeMetrics, 0, len(scopeMetrics))
	for _, sm := range scopeMetrics {
		ms := metricsToPB(sm.Metrics)
		out = append(out, &mpb.ScopeMetrics{
			Scope: &cpb.InstrumentationScope{
				Name:    sm.Scope.Name,
				Version: sm.Scope.Version,
			},
			Metrics: ms,
		})
	}
	return out
}

// metrics returns a slice of OTLP Metric generated from OTEL metrics sdk ones.
func metricsToPB(metrics []metricdata.Metrics) []*mpb.Metric {
	out := make([]*mpb.Metric, 0, len(metrics))
	for _, m := range metrics {
		o, err := metricTypeToPB(m)
		if err != nil {
			goMetrics.IncrCounter(internalMetricTransformFailure, 1)
			continue
		}
		out = append(out, o)
	}
	return out
}

// metricType identifies the instrument type and converts it to OTLP format.
// only float64 values are accepted since the go metrics sink only receives float64 values.
func metricTypeToPB(m metricdata.Metrics) (*mpb.Metric, error) {
	out := &mpb.Metric{
		Name:        m.Name,
		Description: m.Description,
		Unit:        m.Unit,
	}
	switch a := m.Data.(type) {
	case metricdata.Gauge[float64]:
		out.Data = &mpb.Metric_Gauge{
			Gauge: &mpb.Gauge{
				DataPoints: dataPointsToPB(a.DataPoints),
			},
		}
	case metricdata.Sum[float64]:
		if a.Temporality != metricdata.CumulativeTemporality {
			return out, fmt.Errorf("error: %w: %T", errTemporality, a)
		}
		out.Data = &mpb.Metric_Sum{
			Sum: &mpb.Sum{
				AggregationTemporality: mpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				IsMonotonic:            a.IsMonotonic,
				DataPoints:             dataPointsToPB(a.DataPoints),
			},
		}
	case metricdata.Histogram[float64]:
		if a.Temporality != metricdata.CumulativeTemporality {
			return out, fmt.Errorf("error: %w: %T", errTemporality, a)
		}
		out.Data = &mpb.Metric_Histogram{
			Histogram: &mpb.Histogram{
				AggregationTemporality: mpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints:             histogramDataPointsToPB(a.DataPoints),
			},
		}
	default:
		return out, fmt.Errorf("error: %w: %T", errAggregaton, a)
	}
	return out, nil
}

// DataPoints returns a slice of OTLP NumberDataPoint generated from OTEL metrics sdk ones.
func dataPointsToPB(dataPoints []metricdata.DataPoint[float64]) []*mpb.NumberDataPoint {
	out := make([]*mpb.NumberDataPoint, 0, len(dataPoints))
	for _, dp := range dataPoints {
		ndp := &mpb.NumberDataPoint{
			Attributes:        attributesToPB(dp.Attributes.Iter()),
			StartTimeUnixNano: uint64(dp.StartTime.UnixNano()),
			TimeUnixNano:      uint64(dp.Time.UnixNano()),
		}

		ndp.Value = &mpb.NumberDataPoint_AsDouble{
			AsDouble: dp.Value,
		}
		out = append(out, ndp)
	}
	return out
}

// HistogramDataPoints returns a slice of OTLP HistogramDataPoint from OTEL metrics sdk ones.
func histogramDataPointsToPB(dataPoints []metricdata.HistogramDataPoint[float64]) []*mpb.HistogramDataPoint {
	out := make([]*mpb.HistogramDataPoint, 0, len(dataPoints))
	for _, dp := range dataPoints {
		sum := dp.Sum
		hdp := &mpb.HistogramDataPoint{
			Attributes:        attributesToPB(dp.Attributes.Iter()),
			StartTimeUnixNano: uint64(dp.StartTime.UnixNano()),
			TimeUnixNano:      uint64(dp.Time.UnixNano()),
			Count:             dp.Count,
			Sum:               &sum,
			BucketCounts:      dp.BucketCounts,
			ExplicitBounds:    dp.Bounds,
		}
		if v, ok := dp.Min.Value(); ok {
			hdp.Min = &v
		}
		if v, ok := dp.Max.Value(); ok {
			hdp.Max = &v
		}
		out = append(out, hdp)
	}
	return out
}

// attributes transforms items of an attribute iterator into OTLP key-values.
// Currently, labels are only <string, string> key-value pairs.
func attributesToPB(iter attribute.Iterator) []*cpb.KeyValue {
	l := iter.Len()
	if iter.Len() == 0 {
		return nil
	}

	out := make([]*cpb.KeyValue, 0, l)
	for iter.Next() {
		kv := iter.Attribute()
		av := &cpb.AnyValue{
			Value: &cpb.AnyValue_StringValue{
				StringValue: kv.Value.AsString(),
			},
		}
		out = append(out, &cpb.KeyValue{Key: string(kv.Key), Value: av})
	}
	return out
}
