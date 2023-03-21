package telemetry

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-hclog"
	collector_v1metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	v1common "go.opentelemetry.io/proto/otlp/common/v1"
	v1metrics "go.opentelemetry.io/proto/otlp/metrics/v1"
	v1resource "go.opentelemetry.io/proto/otlp/resource/v1"
)

const (
	defaultStreamTimeout  = time.Minute
	defaultReportInterval = 5 * time.Second
	defaultBatchInterval  = 10 * time.Second
)

type Config struct {
	StreamTimeout  time.Duration
	ReportInterval time.Duration
	BatchInterval  time.Duration
	Labels         map[string]string
	Logger         hclog.Logger
	Gatherer       lib.MetricsHandler
	Exporter       Exporter
}

func DefaultConfig() *Config {
	return &Config{
		StreamTimeout:  defaultStreamTimeout,
		ReportInterval: defaultReportInterval,
		BatchInterval:  defaultBatchInterval,
	}
}

type Reporter struct {
	cfg      Config
	filter   *FilterList
	resource *v1resource.Resource

	shutdownOnce sync.Once
	shutdownCh   chan struct{}

	batchedMetrics       map[time.Time][]*v1metrics.Metric
	lastIntervalExported time.Time
	flushCh              chan struct{}
}

func NewReporter(cfg *Config) *Reporter {
	r := &Reporter{
		cfg: *cfg,
		resource: &v1resource.Resource{
			Attributes: make([]*v1common.KeyValue, len(cfg.Labels)),
		},
		filter:         &FilterList{map[string]*regexp.Regexp{}},
		batchedMetrics: make(map[time.Time][]*v1metrics.Metric),
		flushCh:        make(chan struct{}, 1),
	}

	for key, val := range cfg.Labels {
		r.resource.Attributes = append(r.resource.Attributes, &v1common.KeyValue{
			Key:   key,
			Value: &v1common.AnyValue{Value: &v1common.AnyValue_StringValue{StringValue: val}},
		})
	}

	return r
}

func (r *Reporter) Run(ctx context.Context) {
	r.cfg.Logger.Debug("HCP Metrics Reporter starting")

	flushTimer := time.NewTicker(r.cfg.BatchInterval)
	defer flushTimer.Stop()
	for {
		select {
		case <-ctx.Done():
			return

		case <-time.After(r.cfg.ReportInterval):
			r.gatherMetrics()

		case <-flushTimer.C:
			select {
			case r.flushCh <- struct{}{}:
			default:
			}

		case <-r.flushCh:
			flushTimer.Reset(r.cfg.BatchInterval)
			if err := r.flushMetrics(); err != nil {
				// todo handle/log
			}

		}
	}
}

func (r *Reporter) gatherMetrics() {
	intervals := r.cfg.Gatherer.Data()
	if len(intervals) >= 1 {
		// Discard the current interval. We will wait until it is populated to gather it.
		intervals = intervals[:len(intervals)-1]
	}

	for _, interval := range intervals {
		if _, ok := r.batchedMetrics[interval.Interval]; ok {
			continue
		}
		r.batchedMetrics[interval.Interval] = r.goMetricsToOTLP(interval)
	}
	return
}

func (r *Reporter) flushMetrics() error {
	metricsList := make([]*v1metrics.Metric, 0)
	for interval, intervalMetrics := range r.batchedMetrics {
		if len(intervalMetrics) == 0 {
			continue
		}
		metricsList = append(metricsList, intervalMetrics...)
		r.batchedMetrics[interval] = nil
	}
	if len(metricsList) == 0 {
		return nil
	}

	export := &collector_v1metrics.ExportMetricsServiceRequest{
		ResourceMetrics: []*v1metrics.ResourceMetrics{
			{
				Resource: r.resource,
				ScopeMetrics: []*v1metrics.ScopeMetrics{
					{
						Metrics: metricsList,
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.StreamTimeout)
	defer cancel()

	_, err := r.cfg.Exporter.Export(ctx, export)
	if err != nil {
		return fmt.Errorf("failed to export metrics: %w", err)
	}
	return nil
}

func (r *Reporter) goMetricsToOTLP(interval *metrics.IntervalMetrics) []*v1metrics.Metric {
	otlpMetrics := make([]*v1metrics.Metric, 0)
	timestamp := uint64(interval.Interval.UnixNano())

	for _, v := range interval.Gauges {
		if !r.filter.Match(v.Name) {
			continue
		}
		otlpMetrics = append(otlpMetrics, &v1metrics.Metric{
			Name: v.Name,
			Data: &v1metrics.Metric_Gauge{
				Gauge: &v1metrics.Gauge{
					DataPoints: []*v1metrics.NumberDataPoint{
						{
							Attributes:   goMetricsLabelPairsToOTLP(v.Labels),
							TimeUnixNano: timestamp,
							Value:        &v1metrics.NumberDataPoint_AsDouble{AsDouble: float64(v.Value)},
						},
					},
				},
			},
		})
	}

	for _, v := range interval.Counters {
		if !r.filter.Match(v.Name) {
			continue
		}
		otlpMetrics = append(otlpMetrics, &v1metrics.Metric{
			Name: v.Name,
			Data: &v1metrics.Metric_Sum{
				Sum: &v1metrics.Sum{
					DataPoints: []*v1metrics.NumberDataPoint{
						{
							Attributes:   goMetricsLabelPairsToOTLP(v.Labels),
							TimeUnixNano: timestamp,
							Value:        &v1metrics.NumberDataPoint_AsDouble{AsDouble: v.Sum},
						},
					},
				},
			},
		})
	}

	for _, v := range interval.Samples {
		if !r.filter.Match(v.Name) {
			continue
		}
		otlpMetrics = append(otlpMetrics, &v1metrics.Metric{
			Name: v.Name,
			Data: &v1metrics.Metric_Summary{
				Summary: &v1metrics.Summary{
					DataPoints: []*v1metrics.SummaryDataPoint{
						{
							Attributes:   goMetricsLabelPairsToOTLP(v.Labels),
							TimeUnixNano: timestamp,
							Sum:          v.Sum,
							Count:        uint64(v.Count),
						},
					},
				},
			},
		})
	}

	return otlpMetrics
}

func goMetricsLabelPairsToOTLP(labels []metrics.Label) []*v1common.KeyValue {
	kv := make([]*v1common.KeyValue, len(labels))
	for i, label := range labels {
		kv[i] = &v1common.KeyValue{
			Key: label.Name,
			Value: &v1common.AnyValue{
				Value: &v1common.AnyValue_StringValue{
					StringValue: label.Value,
				},
			},
		}
	}
	return kv
}
