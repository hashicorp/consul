package telemetry

import (
	"context"
	"regexp"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

type MetricsExporter struct {
	exp      metric.Exporter
	resource *resource.Resource
	logger   hclog.Logger
	filter   *FilterList
}

func NewMetricsExporter(ctx context.Context, labels map[string]string, endpoint string, logger hclog.Logger, filter []string) (*MetricsExporter, error) {
	exp, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(endpoint), otlpmetrichttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	attr := make([]attribute.KeyValue, len(labels))
	for key, val := range labels {
		attr = append(attr, attribute.KeyValue{
			Key:   attribute.Key(key),
			Value: attribute.StringValue(val),
		})
	}
	res := resource.NewWithAttributes("", attr...)
	f := &FilterList{map[string]*regexp.Regexp{}}
	f.Set(filter)

	m := &MetricsExporter{
		exp:      exp,
		resource: res,
		logger:   logger,
		filter:   f,
	}

	return m, nil
}

func (m *MetricsExporter) Export(ctx context.Context, goMetrics []*metrics.IntervalMetrics) error {
	m.logger.Debug("Exporting metrics original", "intervalMetrics", goMetrics)
	metrics := make([]metricdata.Metrics, 0)
	for _, interval := range goMetrics {
		otlpMetrics := m.goMetricsToOTLP(interval)

		if len(otlpMetrics) > 0 {
			metrics = append(metrics, otlpMetrics...)
		}
	}

	if len(metrics) < 1 {
		return nil
	}

	resourceMetrics := metricdata.ResourceMetrics{
		Resource: m.resource,
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Scope: instrumentation.Scope{
					Name:      "consul-server",
					Version:   "v1",
					SchemaURL: "",
				},
				Metrics: metrics,
			},
		},
	}

	m.logger.Debug("Exporting metrics final", "metrics", resourceMetrics)

	return m.exp.Export(ctx, resourceMetrics)
}

func (m *MetricsExporter) goMetricsToOTLP(interval *metrics.IntervalMetrics) []metricdata.Metrics {
	otlpMetrics := make([]metricdata.Metrics, 0)
	for _, v := range interval.Gauges {
		if !m.filter.Match(v.Name) || v.Name == "" {
			continue
		}

		otlpMetrics = append(otlpMetrics, metricdata.Metrics{
			Name: v.Name,
			Data: metricdata.Gauge[float64]{
				DataPoints: []metricdata.DataPoint[float64]{
					{
						Attributes: goMetricsLabelPairsToOTLP(v.Labels),
						Time:       interval.Interval,
						Value:      float64(v.Value),
					},
				},
			},
		})
	}

	for _, v := range interval.Counters {
		if !m.filter.Match(v.Name) || v.Name == "" {
			continue
		}
		otlpMetrics = append(otlpMetrics, metricdata.Metrics{
			Name: v.Name,
			Data: metricdata.Sum[float64]{
				// TODO: check this
				Temporality: metric.DefaultTemporalitySelector(metric.InstrumentKindCounter),
				DataPoints: []metricdata.DataPoint[float64]{
					{
						Attributes: goMetricsLabelPairsToOTLP(v.Labels),
						Time:       interval.Interval,
						Value:      v.Sum,
					},
				},
			},
		})
	}

	for _, v := range interval.Samples {
		if !m.filter.Match(v.Name) || v.Name == "" {
			continue
		}
		otlpMetrics = append(otlpMetrics, metricdata.Metrics{
			Name: v.Name,
			Data: metricdata.Histogram{
				// TODO: check this
				Temporality: metric.DefaultTemporalitySelector(metric.InstrumentKindHistogram),
				DataPoints: []metricdata.HistogramDataPoint{
					{
						Attributes: goMetricsLabelPairsToOTLP(v.Labels),
						Sum:        v.Sum,
						Min:        metricdata.NewExtrema(v.Min),
						Max:        metricdata.NewExtrema(v.Max),
						Time:       interval.Interval,
						Count:      uint64(v.Count),
					},
				},
			},
		})
	}

	return otlpMetrics
}

func goMetricsLabelPairsToOTLP(labels []metrics.Label) attribute.Set {
	keyValues := make([]attribute.KeyValue, len(labels))
	for i, label := range labels {
		keyValues[i] = attribute.KeyValue{
			Key:   attribute.Key(label.Name),
			Value: attribute.StringValue(label.Value),
		}
	}
	return attribute.NewSet(keyValues...)
}
