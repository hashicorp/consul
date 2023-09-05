// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package usagemetrics

import (
	"testing"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/hashicorp/consul/agent/consul/state"
)

type mockStateProvider struct {
	mock.Mock
}

func (m *mockStateProvider) State() *state.Store {
	retValues := m.Called()
	return retValues.Get(0).(*state.Store)
}

func assertEqualGaugeMaps(t *testing.T, expectedMap, foundMap map[string]metrics.GaugeValue) {
	t.Helper()

	for key := range foundMap {
		if _, ok := expectedMap[key]; !ok {
			t.Errorf("found unexpected gauge key:      %s", key)
		}
	}

	for key, expected := range expectedMap {
		if _, ok := foundMap[key]; !ok {
			t.Errorf("did not find expected gauge key: %s", key)
			continue
		}
		assert.Equal(t, expected, foundMap[key], "gauge key mismatch on %q", key)
	}
}
