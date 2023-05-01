// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lib

import (
	"errors"
	"net"
	"os"
	"testing"

	"github.com/hashicorp/consul/logging"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"
)

func newCfg() TelemetryConfig {
	return TelemetryConfig{
		StatsdAddr:    "statsd.host:1234",
		StatsiteAddr:  "statsite.host:1234",
		DogstatsdAddr: "mydog.host:8125",
		ExtraSinks: []metrics.MetricSink{
			&metrics.BlackholeSink{},
		},
	}
}

func TestConfigureSinks(t *testing.T) {
	cfg := newCfg()
	sinks, err := configureSinks(cfg, nil)
	require.Error(t, err)
	// 4 sinks: statsd, statsite, inmem, extra sink (blackhole)
	require.Equal(t, 4, len(sinks))

	cfg = TelemetryConfig{
		DogstatsdAddr: "",
	}
	_, err = configureSinks(cfg, nil)
	require.NoError(t, err)

}

func TestIsRetriableError(t *testing.T) {
	var err error
	err = multierror.Append(err, errors.New("an error"))
	r := isRetriableError(err)
	require.False(t, r)

	err = multierror.Append(err, &net.DNSError{
		IsNotFound: true,
	})
	r = isRetriableError(err)
	require.True(t, r)
}

func TestInitTelemetryRetrySuccess(t *testing.T) {
	logger, err := logging.Setup(logging.Config{
		LogLevel: "INFO",
	}, os.Stdout)
	require.NoError(t, err)
	cfg := newCfg()
	_, err = InitTelemetry(cfg, logger)
	require.Error(t, err)

	cfg.RetryFailedConfiguration = true
	metricsCfg, err := InitTelemetry(cfg, logger)
	require.NoError(t, err)
	// TODO: we couldn't extract the metrics sinks from the
	// global metrics due to it's limitation
	// fanoutSink := metrics.Default()}
	metricsCfg.cancelFn()
}
