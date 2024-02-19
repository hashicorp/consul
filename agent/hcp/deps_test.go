// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/hcp/telemetry"
)

type mockMetricsClient struct {
	telemetry.MetricsClient
}

func TestSink(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s, err := newSink(ctx, mockMetricsClient{}, &hcpProviderImpl{})

	require.NotNil(t, s)
	require.NoError(t, err)
}
