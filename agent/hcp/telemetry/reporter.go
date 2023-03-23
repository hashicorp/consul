package telemetry

import (
	"context"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-hclog"
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
	Exporter       *MetricsExporter
}

func DefaultConfig() *Config {
	return &Config{
		StreamTimeout:  defaultStreamTimeout,
		ReportInterval: defaultReportInterval,
		BatchInterval:  defaultBatchInterval,
	}
}

type Reporter struct {
	cfg Config

	shutdownOnce sync.Once
	shutdownCh   chan struct{}

	batchedMetrics       map[time.Time]*metrics.IntervalMetrics
	lastIntervalExported time.Time
	flushCh              chan struct{}
}

func NewReporter(cfg *Config) *Reporter {
	r := &Reporter{
		cfg: *cfg,

		batchedMetrics: make(map[time.Time]*metrics.IntervalMetrics),
		flushCh:        make(chan struct{}, 1),
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
				r.cfg.Logger.Error("Failed to flush metrics", "error", err)
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
		r.batchedMetrics[interval.Interval] = interval
	}
	return
}

func (r *Reporter) flushMetrics() error {
	metricsList := make([]*metrics.IntervalMetrics, 0)
	for interval, intervalMetrics := range r.batchedMetrics {
		if intervalMetrics != nil {
			metricsList = append(metricsList, intervalMetrics)
		}
		r.batchedMetrics[interval] = nil
	}
	if len(metricsList) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.StreamTimeout)
	defer cancel()

	return r.cfg.Exporter.Export(ctx, metricsList)
}
