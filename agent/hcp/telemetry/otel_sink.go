package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/go-hclog"

	gometrics "github.com/armon/go-metrics"
	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	otelsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

type OTELSinkOpts struct {
	Reader otelsdk.Reader
	Logger hclog.Logger
	Ctx    context.Context
}

type OTELSink struct {
	spaceReplacer *strings.Replacer
	logger        hclog.Logger
	ctx           context.Context

	meterProvider *otelsdk.MeterProvider
	meter         *otelmetric.Meter

	gaugeInstruments     map[string]*instrument.Float64ObservableGauge
	counterInstruments   map[string]*instrument.Float64Counter
	histogramInstruments map[string]*instrument.Float64Histogram

	mutex sync.Mutex
}

func NewOTELReader(client client.MetricsClient, endpoint string, exportInterval time.Duration) otelsdk.Reader {
	exporter := &OTELExporter{
		client:   client,
		endpoint: endpoint,
	}
	return otelsdk.NewPeriodicReader(exporter, otelsdk.WithInterval(exportInterval))
}

func NewOTELSink(opts *OTELSinkOpts) (*OTELSink, error) {
	if opts.Logger == nil || opts.Reader == nil || opts.Ctx == nil {
		return nil, fmt.Errorf("failed to init OTEL sink: provide valid OTELSinkOpts")
	}

	// Setup OTEL Metrics SDK to aggregate, convert and export metrics periodically.
	res := resource.NewSchemaless()
	meterProvider := otelsdk.NewMeterProvider(otelsdk.WithResource(res), otelsdk.WithReader(opts.Reader))
	meter := meterProvider.Meter("github.com/hashicorp/consul/agent/hcp/telemetry")

	// Init global gauge store.
	initGaugeStore()

	return &OTELSink{
		spaceReplacer:        strings.NewReplacer(" ", "_"),
		logger:               opts.Logger.Named("otel_sink"),
		ctx:                  opts.Ctx,
		meterProvider:        meterProvider,
		meter:                &meter,
		mutex:                sync.Mutex{},
		gaugeInstruments:     make(map[string]*instrument.Float64ObservableGauge, 0),
		counterInstruments:   make(map[string]*instrument.Float64Counter, 0),
		histogramInstruments: make(map[string]*instrument.Float64Histogram, 0),
	}, nil
}

// SetGauge emits a Consul gauge metric.
func (o *OTELSink) SetGauge(key []string, val float32) {
	o.SetGaugeWithLabels(key, val, nil)
}

// AddSample emits a Consul histogram metric.
func (o *OTELSink) AddSample(key []string, val float32) {
	o.AddSampleWithLabels(key, val, nil)
}

// IncrCounter emits a Consul counter metric.
func (o *OTELSink) IncrCounter(key []string, val float32) {
	o.IncrCounterWithLabels(key, val, nil)
}

// AddSampleWithLabels emits a Consul gauge metric that gets
// registed by an OpenTelemetry Histogram instrument.
func (o *OTELSink) SetGaugeWithLabels(key []string, val float32, labels []gometrics.Label) {
	k := o.flattenKey(key, labels)

	// Set value in global Gauge store.
	globalGauges.Store(k, float64(val), toAttributes(labels))

	o.mutex.Lock()
	defer o.mutex.Unlock()

	// If instrument does not exist, create it and register callback to emit last value in global Gauge store.
	if _, ok := o.gaugeInstruments[k]; !ok {
		inst, err := (*o.meter).Float64ObservableGauge(k, instrument.WithFloat64Callback(gaugeCallback(k)))
		if err != nil {
			o.logger.Error("Failed to emit gauge: %w", err)
			return
		}
		o.gaugeInstruments[k] = &inst
	}
}

// AddSampleWithLabels emits a Consul sample metric that gets registed by an OpenTelemetry Histogram instrument.
func (o *OTELSink) AddSampleWithLabels(key []string, val float32, labels []gometrics.Label) {
	k := o.flattenKey(key, labels)

	o.mutex.Lock()
	defer o.mutex.Unlock()

	inst, ok := o.histogramInstruments[k]
	if !ok {
		histogram, err := (*o.meter).Float64Histogram(k)
		if err != nil {
			o.logger.Error("Failed to emit gauge: %w", err)
			return
		}
		inst = &histogram
		o.histogramInstruments[k] = inst
	}

	attrs := toAttributes(labels)
	(*inst).Record(o.ctx, float64(val), attrs...)
}

// IncrCounterWithLabels emits a Consul counter metric that gets registed by an OpenTelemetry Histogram instrument.
func (o *OTELSink) IncrCounterWithLabels(key []string, val float32, labels []gometrics.Label) {
	k := o.flattenKey(key, labels)

	o.mutex.Lock()
	defer o.mutex.Unlock()

	inst, ok := o.counterInstruments[k]
	if !ok {
		counter, err := (*o.meter).Float64Counter(k)
		if err != nil {
			o.logger.Error("Failed to emit gauge: %w", err)
			return
		}

		inst = &counter
		o.counterInstruments[k] = inst
	}

	attrs := toAttributes(labels)
	(*inst).Add(o.ctx, float64(val), attrs...)
}

// EmitKey unsupported.
func (o *OTELSink) EmitKey(key []string, val float32) {}

// flattenKey key along with its labels.
func (o *OTELSink) flattenKey(parts []string, labels []gometrics.Label) string {
	buf := &bytes.Buffer{}
	joined := strings.Join(parts, ".")

	o.spaceReplacer.WriteString(buf, joined)

	return buf.String()
}

// toAttributes converts go metrics Labels into OTEL format []attributes.KeyValue
func toAttributes(labels []gometrics.Label) []attribute.KeyValue {
	if len(labels) == 0 {
		return nil
	}
	attrs := make([]attribute.KeyValue, len(labels))
	for i, label := range labels {
		attrs[i] = attribute.KeyValue{
			Key:   attribute.Key(label.Name),
			Value: attribute.StringValue(label.Value),
		}
	}

	return attrs
}

// gaugeCallback returns a callback which gets called when metrics are collected for export.
// the callback obtains the gauge value from the global gauges.
func gaugeCallback(key string) instrument.Float64Callback {
	// Closures keep a reference to the key string, that get garbage collected when code completes.
	return func(_ context.Context, obs instrument.Float64Observer) error {
		if gauge, ok := globalGauges.LoadAndDelete(key); ok {
			obs.Observe(gauge.Value, gauge.Attributes...)
		}
		return nil
	}
}
