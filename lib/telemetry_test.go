package lib

import (
	"os"
	"testing"

	"github.com/hashicorp/consul/logging"
	"github.com/stretchr/testify/require"
)

func newCfg() TelemetryConfig {
	cfg := TelemetryConfig{
		StatsdAddr:    "statsd.host:1234",
		StatsiteAddr:  "statsite.host:1234",
		DogstatsdAddr: "mydog.host:8125",
	}
	return cfg
}

func TestConfigureSinks(t *testing.T) {
	cfg := newCfg()
	sinks, err := configureSinks(cfg, "hostname", nil)
	require.Error(t, err)
	// 3 sinks: statsd, statsite, inmem
	require.Equal(t, 3, len(sinks))
}

func TestInitTelemetryRetrySuccess(t *testing.T) {
	logger, err := logging.Setup(logging.Config{
		LogLevel: "INFO",
	}, os.Stdout)
	require.NoError(t, err)
	cfg := newCfg()
	_, err = InitTelemetry(cfg, logger)
	require.Error(t, err)

	// TODO: we couldn't extract the metrics sinks from the
	// global metrics due to it's limitation
	// fanoutSink := metrics.Default()}
}
