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

// Store for Gauge values as workaround for async OpenTelemetry Gauge instrument.
var gauges *GlobalGaugeStore

type GaugeValue struct {
	Value      float64
	Attributes []attribute.KeyValue
}

type GlobalGaugeStore struct {
	store map[string]*GaugeValue
	mutex sync.Mutex
}

// LoadAndDelete will read a Gauge value and delete it.
// Within the Gauge callbacks we delete the value once we have registed it with OTEL to ensure
// we only emit a Gauge value once.
func (g *GlobalGaugeStore) LoadAndDelete(key string) (*GaugeValue, bool) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	gauge, ok := g.store[key]

	delete(g.store, key)

	return gauge, ok
}

func (g *GlobalGaugeStore) Store(key string, gauge *GaugeValue) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.store[key] = gauge
}

type OTELSinkOpts struct {
	Reader otelsdk.Reader
	Logger hclog.Logger
	Ctx    context.Context
}

type OTELSink struct {
	spaceReplacer *strings.Replacer
	logger        hclog.Logger
	ctx           context.Context

	meterProvider  *otelsdk.MeterProvider
	meter          *otelmetric.Meter
	exportInterval time.Duration

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

func NewOTELSink(opts *OTELSinkOpts) (gometrics.MetricSink, error) {
	if opts.Logger == nil || opts.Reader == nil || opts.Ctx == nil {
		return nil, fmt.Errorf("failed to init OTEL sink: provide valid OTELSinkOpts")
	}

	// Setup OTEL Metrics SDK to aggregate, convert and export metrics periodically.
	res := resource.NewSchemaless()
	meterProvider := otelsdk.NewMeterProvider(otelsdk.WithResource(res), otelsdk.WithReader(opts.Reader))
	meter := meterProvider.Meter("github.com/hashicorp/consul/agent/hcp/telemetry")

	// Init global gauge store.
	gauges = &GlobalGaugeStore{
		store: make(map[string]*GaugeValue, 0),
		mutex: sync.Mutex{},
	}

	return &OTELSink{
		meterProvider:        meterProvider,
		meter:                &meter,
		spaceReplacer:        strings.NewReplacer(" ", "_"),
		ctx:                  opts.Ctx,
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
	g := &GaugeValue{
		Value:      float64(val),
		Attributes: toAttributes(labels),
	}
	gauges.Store(k, g)

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

func gaugeCallback(key string) instrument.Float64Callback {
	// Closures keep a reference to the key string, so we don't have to worry about it.
	// These get garbage collected as the closure completes.
	return func(_ context.Context, obs instrument.Float64Observer) error {
		if gauge, ok := gauges.LoadAndDelete(key); ok {
			obs.Observe(gauge.Value, gauge.Attributes...)
		}
		return nil
	}
}
