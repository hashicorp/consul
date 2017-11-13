package sarama

import (
	"os"
	"testing"

	"github.com/rcrowley/go-metrics"
)

func TestDefaultConfigValidates(t *testing.T) {
	config := NewConfig()
	if err := config.Validate(); err != nil {
		t.Error(err)
	}
	if config.MetricRegistry == nil {
		t.Error("Expected non nil metrics.MetricRegistry, got nil")
	}
}

func TestInvalidClientIDConfigValidates(t *testing.T) {
	config := NewConfig()
	config.ClientID = "foo:bar"
	if err := config.Validate(); string(err.(ConfigurationError)) != "ClientID is invalid" {
		t.Error("Expected invalid ClientID, got ", err)
	}
}

func TestEmptyClientIDConfigValidates(t *testing.T) {
	config := NewConfig()
	config.ClientID = ""
	if err := config.Validate(); string(err.(ConfigurationError)) != "ClientID is invalid" {
		t.Error("Expected invalid ClientID, got ", err)
	}
}

func TestNetConfigValidates(t *testing.T) {
	tests := []struct {
		name string
		cfg  func(*Config) // resorting to using a function as a param because of internal composite structs
		err  string
	}{
		{
			"OpenRequests",
			func(cfg *Config) {
				cfg.Net.MaxOpenRequests = 0
			},
			"Net.MaxOpenRequests must be > 0"},
		{"DialTimeout",
			func(cfg *Config) {
				cfg.Net.DialTimeout = 0
			},
			"Net.DialTimeout must be > 0"},
		{"ReadTimeout",
			func(cfg *Config) {
				cfg.Net.ReadTimeout = 0
			},
			"Net.ReadTimeout must be > 0"},
		{"WriteTimeout",
			func(cfg *Config) {
				cfg.Net.WriteTimeout = 0
			},
			"Net.WriteTimeout must be > 0"},
		{"KeepAlive",
			func(cfg *Config) {
				cfg.Net.KeepAlive = -1
			},
			"Net.KeepAlive must be >= 0"},
		{"SASL.User",
			func(cfg *Config) {
				cfg.Net.SASL.Enable = true
				cfg.Net.SASL.User = ""
			},
			"Net.SASL.User must not be empty when SASL is enabled"},
		{"SASL.Password",
			func(cfg *Config) {
				cfg.Net.SASL.Enable = true
				cfg.Net.SASL.User = "user"
				cfg.Net.SASL.Password = ""
			},
			"Net.SASL.Password must not be empty when SASL is enabled"},
	}

	for i, test := range tests {
		c := NewConfig()
		test.cfg(c)
		if err := c.Validate(); string(err.(ConfigurationError)) != test.err {
			t.Errorf("[%d]:[%s] Expected %s, Got %s\n", i, test.name, test.err, err)
		}
	}
}

func TestMetadataConfigValidates(t *testing.T) {
	tests := []struct {
		name string
		cfg  func(*Config) // resorting to using a function as a param because of internal composite structs
		err  string
	}{
		{
			"Retry.Max",
			func(cfg *Config) {
				cfg.Metadata.Retry.Max = -1
			},
			"Metadata.Retry.Max must be >= 0"},
		{"Retry.Backoff",
			func(cfg *Config) {
				cfg.Metadata.Retry.Backoff = -1
			},
			"Metadata.Retry.Backoff must be >= 0"},
		{"RefreshFrequency",
			func(cfg *Config) {
				cfg.Metadata.RefreshFrequency = -1
			},
			"Metadata.RefreshFrequency must be >= 0"},
	}

	for i, test := range tests {
		c := NewConfig()
		test.cfg(c)
		if err := c.Validate(); string(err.(ConfigurationError)) != test.err {
			t.Errorf("[%d]:[%s] Expected %s, Got %s\n", i, test.name, test.err, err)
		}
	}
}

func TestProducerConfigValidates(t *testing.T) {
	tests := []struct {
		name string
		cfg  func(*Config) // resorting to using a function as a param because of internal composite structs
		err  string
	}{
		{
			"MaxMessageBytes",
			func(cfg *Config) {
				cfg.Producer.MaxMessageBytes = 0
			},
			"Producer.MaxMessageBytes must be > 0"},
		{"RequiredAcks",
			func(cfg *Config) {
				cfg.Producer.RequiredAcks = -2
			},
			"Producer.RequiredAcks must be >= -1"},
		{"Timeout",
			func(cfg *Config) {
				cfg.Producer.Timeout = 0
			},
			"Producer.Timeout must be > 0"},
		{"Partitioner",
			func(cfg *Config) {
				cfg.Producer.Partitioner = nil
			},
			"Producer.Partitioner must not be nil"},
		{"Flush.Bytes",
			func(cfg *Config) {
				cfg.Producer.Flush.Bytes = -1
			},
			"Producer.Flush.Bytes must be >= 0"},
		{"Flush.Messages",
			func(cfg *Config) {
				cfg.Producer.Flush.Messages = -1
			},
			"Producer.Flush.Messages must be >= 0"},
		{"Flush.Frequency",
			func(cfg *Config) {
				cfg.Producer.Flush.Frequency = -1
			},
			"Producer.Flush.Frequency must be >= 0"},
		{"Flush.MaxMessages",
			func(cfg *Config) {
				cfg.Producer.Flush.MaxMessages = -1
			},
			"Producer.Flush.MaxMessages must be >= 0"},
		{"Flush.MaxMessages with Producer.Flush.Messages",
			func(cfg *Config) {
				cfg.Producer.Flush.MaxMessages = 1
				cfg.Producer.Flush.Messages = 2
			},
			"Producer.Flush.MaxMessages must be >= Producer.Flush.Messages when set"},
		{"Flush.Retry.Max",
			func(cfg *Config) {
				cfg.Producer.Retry.Max = -1
			},
			"Producer.Retry.Max must be >= 0"},
		{"Flush.Retry.Backoff",
			func(cfg *Config) {
				cfg.Producer.Retry.Backoff = -1
			},
			"Producer.Retry.Backoff must be >= 0"},
	}

	for i, test := range tests {
		c := NewConfig()
		test.cfg(c)
		if err := c.Validate(); string(err.(ConfigurationError)) != test.err {
			t.Errorf("[%d]:[%s] Expected %s, Got %s\n", i, test.name, test.err, err)
		}
	}
}

func TestLZ4ConfigValidation(t *testing.T) {
	config := NewConfig()
	config.Producer.Compression = CompressionLZ4
	if err := config.Validate(); string(err.(ConfigurationError)) != "lz4 compression requires Version >= V0_10_0_0" {
		t.Error("Expected invalid lz4/kakfa version error, got ", err)
	}
	config.Version = V0_10_0_0
	if err := config.Validate(); err != nil {
		t.Error("Expected lz4 to work, got ", err)
	}
}

// This example shows how to integrate with an existing registry as well as publishing metrics
// on the standard output
func ExampleConfig_metrics() {
	// Our application registry
	appMetricRegistry := metrics.NewRegistry()
	appGauge := metrics.GetOrRegisterGauge("m1", appMetricRegistry)
	appGauge.Update(1)

	config := NewConfig()
	// Use a prefix registry instead of the default local one
	config.MetricRegistry = metrics.NewPrefixedChildRegistry(appMetricRegistry, "sarama.")

	// Simulate a metric created by sarama without starting a broker
	saramaGauge := metrics.GetOrRegisterGauge("m2", config.MetricRegistry)
	saramaGauge.Update(2)

	metrics.WriteOnce(appMetricRegistry, os.Stdout)
	// Output:
	// gauge m1
	//   value:               1
	// gauge sarama.m2
	//   value:               2
}
