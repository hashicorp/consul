package telemetry

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	cpb "go.opentelemetry.io/proto/otlp/common/v1"
	mpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	rpb "go.opentelemetry.io/proto/otlp/resource/v1"
)

// TransformOTLP returns an OTLP ResourceMetrics generated from OTEL metrics. If rm
// contains invalid ScopeMetrics, an error will be returned along with an OTLP
// ResourceMetrics that contains partial OTLP ScopeMetrics.
func transformOTLP(rm *metricdata.ResourceMetrics) (*mpb.ResourceMetrics, error) {
	sms, err := scopeMetrics(rm.ScopeMetrics)
	return &mpb.ResourceMetrics{
		Resource: &rpb.Resource{
			Attributes: attributes(rm.Resource.Iter()),
		},
		ScopeMetrics: sms,
	}, err
}

// scopeMetrics returns a slice of OTLP ScopeMetrics.
func scopeMetrics(scopeMetrics []metricdata.ScopeMetrics) ([]*mpb.ScopeMetrics, error) {
	var merr *multierror.Error
	out := make([]*mpb.ScopeMetrics, 0, len(scopeMetrics))
	for _, sm := range scopeMetrics {
		ms, err := metrics(sm.Metrics)
		if err != nil {
			merr = multierror.Append(merr, err)
		}

		out = append(out, &mpb.ScopeMetrics{
			Scope: &cpb.InstrumentationScope{
				Name:    sm.Scope.Name,
				Version: sm.Scope.Version,
			},
			Metrics: ms,
		})
	}
	return out, merr
}

// metrics returns a slice of OTLP Metric generated from OTEL metrics sdk ones.
func metrics(metrics []metricdata.Metrics) ([]*mpb.Metric, error) {
	var merr *multierror.Error
	out := make([]*mpb.Metric, 0, len(metrics))
	for _, m := range metrics {
		o, err := metricType(m)
		if err != nil {
			merr = multierror.Append(merr, err)
			continue
		}
		out = append(out, o)
	}
	return out, merr
}

// metricType identifies the instrument type and converts it to OTLP format.
// only float64 values are accepted since the go metrics sink only receives float64 values.
func metricType(m metricdata.Metrics) (*mpb.Metric, error) {
	var err error
	out := &mpb.Metric{
		Name:        m.Name,
		Description: m.Description,
		Unit:        string(m.Unit),
	}
	switch a := m.Data.(type) {
	case metricdata.Gauge[float64]:
		out.Data = &mpb.Metric_Gauge{
			Gauge: &mpb.Gauge{
				DataPoints: dataPoints(a.DataPoints),
			},
		}
	case metricdata.Sum[float64]:
		if a.Temporality != metricdata.CumulativeTemporality {
			return out, fmt.Errorf("%s: %T", "unsupported temporality", a)
		}
		out.Data = &mpb.Metric_Sum{
			Sum: &mpb.Sum{
				AggregationTemporality: mpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				IsMonotonic:            a.IsMonotonic,
				DataPoints:             dataPoints(a.DataPoints),
			},
		}
	case metricdata.Histogram[float64]:
		if a.Temporality != metricdata.CumulativeTemporality {
			return out, fmt.Errorf("%s: %T", "unsupported temporality", a)
		}
		out.Data = &mpb.Metric_Histogram{
			Histogram: &mpb.Histogram{
				AggregationTemporality: mpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints:             histogramDataPoints(a.DataPoints),
			},
		}
	default:
		return out, fmt.Errorf("%s: %T", "unknown aggregation", a)
	}
	return out, err
}

// DataPoints returns a slice of OTLP NumberDataPoint generated from OTEL metrics sdk ones.
func dataPoints(dataPoints []metricdata.DataPoint[float64]) []*mpb.NumberDataPoint {
	out := make([]*mpb.NumberDataPoint, 0, len(dataPoints))
	for _, dp := range dataPoints {
		ndp := &mpb.NumberDataPoint{
			Attributes:        attributes(dp.Attributes.Iter()),
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
func histogramDataPoints(dataPoints []metricdata.HistogramDataPoint[float64]) []*mpb.HistogramDataPoint {
	out := make([]*mpb.HistogramDataPoint, 0, len(dataPoints))
	for _, dp := range dataPoints {
		sum := dp.Sum
		hdp := &mpb.HistogramDataPoint{
			Attributes:        attributes(dp.Attributes.Iter()),
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
func attributes(iter attribute.Iterator) []*cpb.KeyValue {
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
