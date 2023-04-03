package telemetry

import (
	"context"
	"fmt"
	"regexp"

	"github.com/armon/go-metrics"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

type Exporter interface {
	Export(ctx context.Context, goMetrics []*metrics.IntervalMetrics) error
	ConvertToOTLP(goMetrics []*metrics.IntervalMetrics) *metricdata.ResourceMetrics
}

type MetricsExporterConfig struct {
	Labels  map[string]string
	Logger  hclog.Logger
	Filters []string
	Client  hcpclient.Client
}

type MetricsExporter struct {
	client   hcpclient.Client
	resource *resource.Resource
	logger   hclog.Logger
	filter   *FilterList
}

// NewMetricsExporter returns a Metrics exporter with labels to tag metrics
// and filters to only export allowed metrics.
func NewMetricsExporter(cfg *MetricsExporterConfig) (*MetricsExporter, error) {
	if cfg.Client == nil || cfg.Logger == nil {
		return nil, fmt.Errorf("HCP client and a logger are required")
	}

	attr := make([]attribute.KeyValue, 0, len(cfg.Labels))
	for key, val := range cfg.Labels {
		attr = append(attr, attribute.KeyValue{
			Key:   attribute.Key(key),
			Value: attribute.StringValue(val),
		})
	}

	f := &FilterList{map[string]*regexp.Regexp{}}
	if err := f.Set(cfg.Filters); err != nil {
		return nil, fmt.Errorf("invalid regex: %v", err.Error())
	}

	res := resource.NewWithAttributes("", attr...)

	m := &MetricsExporter{
		resource: res,
		logger:   cfg.Logger,
		filter:   f,
		client:   cfg.Client,
	}

	return m, nil
}

// Export filters and converts go metrics into OTLP format
// It calls the HCP client to send the metrics to the HCP Metrics Gateway
// via an authenticated HTTP request to the configured endpoint.
func (m *MetricsExporter) Export(ctx context.Context, goMetrics []*metrics.IntervalMetrics) error {
	r := m.ConvertToOTLP(goMetrics)

	return m.client.ExportMetrics(ctx, r)
}

// ConvertToOTLP creates an OTLP request with given goMetrics.
func (m *MetricsExporter) ConvertToOTLP(goMetrics []*metrics.IntervalMetrics) *metricdata.ResourceMetrics {
	return &metricdata.ResourceMetrics{
		Resource: m.resource,
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Scope: instrumentation.Scope{
					Name:    "github.com/hashicorp/consul/agent/hcp/client/telemetry",
					Version: "v1",
				},
				Metrics: m.goMetricsToOTLP(goMetrics),
			},
		},
	}

}

// goMetricsToOTLP converts go metrics data to OTLP metrics format.
func (m *MetricsExporter) goMetricsToOTLP(goMetrics []*metrics.IntervalMetrics) []metricdata.Metrics {
	otlpMetrics := make([]metricdata.Metrics, 0)
	for _, interval := range goMetrics {
		for _, v := range interval.Gauges {
			if !m.filter.Match(v.Name) {
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
			if !m.filter.Match(v.Name) {
				continue
			}
			otlpMetrics = append(otlpMetrics, metricdata.Metrics{
				Name: v.Name,
				Data: metricdata.Sum[float64]{
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
			if !m.filter.Match(v.Name) {
				continue
			}
			otlpMetrics = append(otlpMetrics, metricdata.Metrics{
				Name: v.Name,
				Data: metricdata.Histogram{
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
	}

	return otlpMetrics
}

// goMetricsToOTLP converts labels to OTLP labels to set attributes.
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
