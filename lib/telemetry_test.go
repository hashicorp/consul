// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lib

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"

	hcptelemetry "github.com/hashicorp/consul/agent/hcp/telemetry"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
)

func newCfg() TelemetryConfig {
	opts := &hcptelemetry.OTELSinkOpts{
		Logger: hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		Reader: metric.NewManualReader(),
		Ctx:    context.Background(),
	}

	return TelemetryConfig{
		StatsdAddr:    "statsd.host:1234",
		StatsiteAddr:  "statsite.host:1234",
		DogstatsdAddr: "mydog.host:8125",
		HCPSinkOpts:   opts,
	}
}

func TestConfigureSinks(t *testing.T) {
	cfg := newCfg()
	sinks, err := configureSinks(cfg, nil)
	require.Error(t, err)
	// 4 sinks: statsd, statsite, inmem, hcp
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
