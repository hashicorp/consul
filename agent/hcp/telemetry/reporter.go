package telemetry

import (
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/hashicorp/go-hclog"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	collector_v1metrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	v1common "go.opentelemetry.io/proto/otlp/common/v1"
	v1metrics "go.opentelemetry.io/proto/otlp/metrics/v1"
	v1resource "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc"
)

const (
	defaultStreamTimeout = time.Minute
)

type Config struct {
	StreamTimeout  time.Duration
	ReportInterval time.Duration
	BatchInterval  time.Duration
	Labels         map[string]string
	Logger         hclog.Logger
}

func DefaultConfig() *Config {
	return &Config{
		StreamTimeout:  defaultStreamTimeout,
		ReportInterval: 5 * time.Second,
		BatchInterval:  10 * time.Second,
		Labels:         map[string]string{},
	}
}

type Reporter struct {
	cfg      Config
	gatherer prometheus.Gatherer
	exporter collector_v1metrics.MetricsServiceClient
	filter   *FilterList
	resource *v1resource.Resource
	logger   hclog.Logger

	shutdownOnce sync.Once
	shutdownCh   chan struct{}

	batchedMetrics []*v1metrics.Metric
	flushCh        chan struct{}
}

type loggingExporter struct{}

func (l *loggingExporter) Export(_ context.Context, req *collector_v1metrics.ExportMetricsServiceRequest, _ ...grpc.CallOption) (*collector_v1metrics.ExportMetricsServiceResponse, error) {
	spew.Dump(req.ResourceMetrics)
	return nil, nil
}

func NewReporter(cfg *Config) *Reporter {
	if cfg == nil {
		return nil
	}

	r := &Reporter{
		cfg: *cfg,
		resource: &v1resource.Resource{
			Attributes: make([]*v1common.KeyValue, len(cfg.Labels)),
		},
		logger:     cfg.Logger,
		gatherer:   prometheus.DefaultGatherer,
		exporter:   &loggingExporter{},
		filter:     &FilterList{map[string]*regexp.Regexp{}},
		shutdownCh: make(chan struct{}),
		flushCh:    make(chan struct{}, 1),
	}

	if err := r.filter.Set([]string{
		"raft_apply$",
	}); err != nil {
		panic(err)
	}

	for key, val := range cfg.Labels {
		r.resource.Attributes = append(r.resource.Attributes, &v1common.KeyValue{
			Key:   key,
			Value: &v1common.AnyValue{Value: &v1common.AnyValue_StringValue{StringValue: val}},
		})
	}

	go r.run()
	return r
}

func (r *Reporter) Stop() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownCh)
	})
}

func (r *Reporter) Flush() {
	r.triggerFlush()
}

func (r *Reporter) run() {
	flushTimer := time.NewTicker(r.cfg.BatchInterval)
	defer flushTimer.Stop()
	for {
		select {
		case <-time.After(r.cfg.ReportInterval):
			metrics, err := r.gatherMetrics()
			if err != nil {
				//todo handle
				continue
			}

			r.batchedMetrics = append(r.batchedMetrics, metrics...)
		case <-flushTimer.C:
			r.triggerFlush()
		case <-r.flushCh:
			flushTimer.Reset(r.cfg.BatchInterval)
			if err := r.flushMetrics(); err != nil {
				//todo handle/log
			}
		case <-r.shutdownCh:
			if err := r.flushMetrics(); err != nil {
				//todo handle/log
			}
			return
		}
	}

}

func (r *Reporter) gatherMetrics() ([]*v1metrics.Metric, error) {
	timestamp := uint64(time.Now().UnixNano())
	fams, err := r.gatherer.Gather()
	if err != nil {
		return nil, err
	}

	var metrics []*v1metrics.Metric
	for _, fam := range fams {
		if r.filter.Match(fam.GetName()) {
			metrics = append(metrics, promMetricsFamilyToOTLP(fam, timestamp)...)
		}
	}

	return metrics, nil
}

func (r *Reporter) flushMetrics() error {
	if r.batchedMetrics == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.StreamTimeout)
	defer cancel()

	data := &v1metrics.ResourceMetrics{
		Resource: r.resource,
		ScopeMetrics: []*v1metrics.ScopeMetrics{
			{
				// todo add instrumentation scope?
				Metrics: r.batchedMetrics,
			},
		},
	}
	export := &collector_v1metrics.ExportMetricsServiceRequest{
		ResourceMetrics: []*v1metrics.ResourceMetrics{data},
	}

	_, err := r.exporter.Export(ctx, export)
	if err != nil {
		return err
	}
	r.batchedMetrics = nil
	return nil
}

func (r *Reporter) triggerFlush() {
	select {
	case r.flushCh <- struct{}{}:
	default:
	}
}

func promMetricsFamilyToOTLP(mf *io_prometheus_client.MetricFamily, timestamp uint64) []*v1metrics.Metric {
	oltpMetrics := make([]*v1metrics.Metric, len(mf.Metric))
	for i, m := range mf.Metric {
		oltpMetrics[i] = &v1metrics.Metric{
			Name: mf.GetName(),
			//		Description: mf.GetHelp(),
		}
		if m.GetTimestampMs() > 0 {
			timestamp = uint64(1000000) * uint64(m.GetTimestampMs())
		}
		switch mf.GetType() {
		case io_prometheus_client.MetricType_COUNTER:
			data := []*v1metrics.NumberDataPoint{
				{
					Attributes:   promLabelPairsToKeyVal(m.Label),
					TimeUnixNano: timestamp,
					Value:        &v1metrics.NumberDataPoint_AsDouble{AsDouble: m.GetCounter().GetValue()},
				},
			}
			oltpMetrics[i].Data = &v1metrics.Metric_Gauge{Gauge: &v1metrics.Gauge{
				DataPoints: data,
			}}
		case io_prometheus_client.MetricType_GAUGE:
			data := []*v1metrics.NumberDataPoint{
				{
					Attributes:   promLabelPairsToKeyVal(m.Label),
					TimeUnixNano: timestamp,
					Value:        &v1metrics.NumberDataPoint_AsDouble{AsDouble: m.GetGauge().GetValue()},
				},
			}
			oltpMetrics[i].Data = &v1metrics.Metric_Gauge{Gauge: &v1metrics.Gauge{
				DataPoints: data,
			}}
		case io_prometheus_client.MetricType_SUMMARY:
			data := []*v1metrics.SummaryDataPoint{
				{
					Attributes:     promLabelPairsToKeyVal(m.Label),
					TimeUnixNano:   timestamp,
					Sum:            m.GetSummary().GetSampleSum(),
					Count:          m.GetSummary().GetSampleCount(),
					QuantileValues: make([]*v1metrics.SummaryDataPoint_ValueAtQuantile, len(m.GetSummary().GetQuantile())),
				},
			}
			for qi, quant := range m.GetSummary().GetQuantile() {
				data[0].QuantileValues[qi] = &v1metrics.SummaryDataPoint_ValueAtQuantile{
					Quantile: quant.GetQuantile(),
					Value:    quant.GetValue(),
				}
			}

			oltpMetrics[i].Data = &v1metrics.Metric_Summary{Summary: &v1metrics.Summary{
				DataPoints: data,
			}}
		case io_prometheus_client.MetricType_HISTOGRAM:
		}
	}

	return oltpMetrics
}

func promLabelPairsToKeyVal(labels []*io_prometheus_client.LabelPair) []*v1common.KeyValue {
	kv := make([]*v1common.KeyValue, len(labels))
	for i, label := range labels {
		kv[i] = &v1common.KeyValue{
			Key: label.GetName(),
			Value: &v1common.AnyValue{
				Value: &v1common.AnyValue_StringValue{
					StringValue: label.GetValue(),
				},
			},
		}
	}
	return kv
}
