package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	gometrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/go-hclog"

	"go.opentelemetry.io/otel/attribute"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	otelsdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

const defaultExportInterval = 10 * time.Second

// Store for Gauge values as workaround for async OpenTelemetry Gauge instrument.
var gauges sync.Map = sync.Map{}

type GaugeValue struct {
	Value  float64
	Labels []attribute.KeyValue
}

type OTELSinkOpts struct {
	Endpoint       string
	Reader         otelsdk.Reader
	Logger         hclog.Logger
	ExportInterval time.Duration
	Ctx            context.Context
}

type OTELSink struct {
	spaceReplacer *strings.Replacer
	logger        hclog.Logger
	ctx           context.Context

	meterProvider  *otelsdk.MeterProvider
	meter          *otelmetric.Meter
	exportInterval time.Duration

	gaugeInstruments     sync.Map
	counterInstruments   sync.Map
	histogramInstruments sync.Map
}

func NewOTELReader(client client.MetricsClient) otelsdk.Reader {
	exp := &OTELExporter{
		client: client,
	}
	return otelsdk.NewPeriodicReader(exp, otelsdk.WithInterval(defaultExportInterval))
}

func NewOTELSink(opts *OTELSinkOpts) (gometrics.MetricSink, error) {
	if opts.Logger == nil || opts.Reader == nil || opts.Endpoint == "" || opts.Ctx == nil {
		return nil, fmt.Errorf("failed to init OTEL sink: provide valid OTELSinkOpts")
	}

	// Setup OTEL Metrics SDK to aggregate, convert and export metrics periodically.
	res := resource.NewSchemaless()
	meterProvider := otelsdk.NewMeterProvider(otelsdk.WithResource(res), otelsdk.WithReader(opts.Reader))
	meter := meterProvider.Meter("github.com/hashicorp/consul/agent/hcp/telemetry")

	return &OTELSink{
		meterProvider: meterProvider,
		meter:         &meter,
		spaceReplacer: strings.NewReplacer(" ", "_"),
		ctx:           opts.Ctx,
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
		Value:  float64(val),
		Labels: toAttributes(labels),
	}
	gauges.Store(k, g)

	// If instrument does not exist, create it and register callback to get last value in global Gauge store.
	if _, ok := o.gaugeInstruments.Load(k); !ok {
		inst, err := (*o.meter).Float64ObservableGauge(k, instrument.WithFloat64Callback(gaugeCallback(k)))
		if err != nil {
			o.logger.Error("Failed to emit gauge: %w", err)
			return
		}
		o.gaugeInstruments.Store(k, &inst)
	}
}

// AddSampleWithLabels emits a Consul sample metric that gets registed by an OpenTelemetry Histogram instrument.
func (o *OTELSink) AddSampleWithLabels(key []string, val float32, labels []gometrics.Label) {
	k := o.flattenKey(key, labels)
	var inst *instrument.Float64Histogram
	v, ok := o.histogramInstruments.Load(k)
	if !ok {
		v, err := (*o.meter).Float64Histogram(k)
		if err != nil {
			o.logger.Error("Failed to emit gauge: %w", err)
			return
		}
		inst = &v
		o.histogramInstruments.Store(k, v)
	} else {
		inst = v.(*instrument.Float64Histogram)
	}

	attrs := toAttributes(labels)
	(*inst).Record(o.ctx, float64(val), attrs...)
}

// IncrCounterWithLabels emits a Consul counter metric that gets registed by an OpenTelemetry Histogram instrument.
func (o *OTELSink) IncrCounterWithLabels(key []string, val float32, labels []gometrics.Label) {
	k := o.flattenKey(key, labels)
	var inst *instrument.Float64Counter
	v, ok := o.histogramInstruments.Load(k)
	if !ok {
		v, err := (*o.meter).Float64Counter(k)
		if err != nil {
			o.logger.Error("Failed to emit gauge: %w", err)
			return
		}
		inst = &v
		o.histogramInstruments.Store(k, v)
	} else {
		inst = v.(*instrument.Float64Counter)
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
		if val, ok := gauges.LoadAndDelete(key); ok {
			v := val.(*GaugeValue)
			obs.Observe(v.Value, v.Labels...)
		}
		return nil
	}
}
