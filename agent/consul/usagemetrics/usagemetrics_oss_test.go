// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package usagemetrics

import (
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/sdk/testutil"
)

func newStateStore() (*state.Store, error) {
	return state.NewStateStore(nil), nil
}

type testCase struct {
	modfiyStateStore func(t *testing.T, s *state.Store)
	getMembersFunc   getMembersFunc
	expectedGauges   map[string]metrics.GaugeValue
}

var baseCases = map[string]testCase{
	"empty-state": {
		expectedGauges: map[string]metrics.GaugeValue{
			// --- node ---
			"consul.usage.test.consul.state.nodes;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.nodes",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.nodes;datacenter=dc1": {
				Name:   "consul.usage.test.state.nodes",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- peering ---
			"consul.usage.test.consul.state.peerings;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.peerings",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.peerings;datacenter=dc1": {
				Name:   "consul.usage.test.state.peerings",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- member ---
			"consul.usage.test.consul.members.clients;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.members.clients",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.members.clients;datacenter=dc1": {
				Name:   "consul.usage.test.members.clients",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.consul.members.servers;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.members.servers",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.members.servers;datacenter=dc1": {
				Name:   "consul.usage.test.members.servers",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- service ---
			"consul.usage.test.consul.state.services;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.services",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.services;datacenter=dc1": {
				Name:   "consul.usage.test.state.services",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.consul.state.service_instances;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.service_instances",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.service_instances;datacenter=dc1": {
				Name:   "consul.usage.test.state.service_instances",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- service mesh ---
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=connect-proxy": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-proxy"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-proxy": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-proxy"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=terminating-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=terminating-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=ingress-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=ingress-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=api-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=api-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=mesh-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=mesh-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh-gateway"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=connect-native": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-native"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-native": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-native"},
				},
			},
			"consul.usage.test.state.billable_service_instances;datacenter=dc1": {
				Name:  "consul.usage.test.state.billable_service_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
				},
			},
			// --- kv ---
			"consul.usage.test.consul.state.kv_entries;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.kv_entries",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.kv_entries;datacenter=dc1": {
				Name:   "consul.usage.test.state.kv_entries",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- config entries ---
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-intentions": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-intentions"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-intentions": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-intentions"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-resolver": { // Legacy
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-resolver"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-resolver": {
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-resolver"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-router": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-router"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-router": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-router"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-defaults": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-defaults"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-defaults": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-defaults"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=ingress-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=ingress-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-splitter": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-splitter"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-splitter": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-splitter"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=mesh": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=mesh": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=proxy-defaults": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "proxy-defaults"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=proxy-defaults": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "proxy-defaults"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=terminating-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=terminating-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=exported-services": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "exported-services"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=exported-services": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "exported-services"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=sameness-group": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "sameness-group"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=sameness-group": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "sameness-group"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=api-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=api-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=bound-api-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "bound-api-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=bound-api-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "bound-api-gateway"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=inline-certificate": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "inline-certificate"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=inline-certificate": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "inline-certificate"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=http-route": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "http-route"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=http-route": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "http-route"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=tcp-route": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "tcp-route"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=tcp-route": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "tcp-route"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=jwt-provider": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "jwt-provider"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=jwt-provider": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "jwt-provider"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=control-plane-request-limit": {
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "control-plane-request-limit"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=control-plane-request-limit": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "control-plane-request-limit"},
				},
			},
		},
		getMembersFunc: func() []serf.Member { return []serf.Member{} },
	},
	"nodes": {
		modfiyStateStore: func(t *testing.T, s *state.Store) {
			require.NoError(t, s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
			require.NoError(t, s.EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
		},
		getMembersFunc: func() []serf.Member {
			return []serf.Member{
				{
					Name:   "foo",
					Tags:   map[string]string{"role": "consul"},
					Status: serf.StatusAlive,
				},
				{
					Name:   "bar",
					Tags:   map[string]string{"role": "consul"},
					Status: serf.StatusAlive,
				},
			}
		},
		expectedGauges: map[string]metrics.GaugeValue{
			// --- node ---
			"consul.usage.test.consul.state.nodes;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.nodes",
				Value:  2,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.nodes;datacenter=dc1": {
				Name:   "consul.usage.test.state.nodes",
				Value:  2,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- peering ---
			"consul.usage.test.consul.state.peerings;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.peerings",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.peerings;datacenter=dc1": {
				Name:   "consul.usage.test.state.peerings",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- member ---
			"consul.usage.test.consul.members.servers;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.members.servers",
				Value:  2,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.members.servers;datacenter=dc1": {
				Name:   "consul.usage.test.members.servers",
				Value:  2,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.consul.members.clients;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.members.clients",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.members.clients;datacenter=dc1": {
				Name:   "consul.usage.test.members.clients",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- service ---
			"consul.usage.test.consul.state.services;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.services",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.services;datacenter=dc1": {
				Name:   "consul.usage.test.state.services",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.consul.state.service_instances;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.service_instances",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.service_instances;datacenter=dc1": {
				Name:   "consul.usage.test.state.service_instances",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- service mesh ---
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=connect-proxy": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-proxy"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-proxy": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-proxy"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=terminating-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=terminating-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=ingress-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=ingress-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=api-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=api-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=mesh-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh-gateway"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=mesh-gateway": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh-gateway"},
				},
			},
			"consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=connect-native": { // Legacy
				Name:  "consul.usage.test.consul.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-native"},
				},
			},
			"consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-native": {
				Name:  "consul.usage.test.state.connect_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "connect-native"},
				},
			},
			"consul.usage.test.state.billable_service_instances;datacenter=dc1": {
				Name:  "consul.usage.test.state.billable_service_instances",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
				},
			},
			// --- kv ---
			"consul.usage.test.consul.state.kv_entries;datacenter=dc1": { // Legacy
				Name:   "consul.usage.test.consul.state.kv_entries",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			"consul.usage.test.state.kv_entries;datacenter=dc1": {
				Name:   "consul.usage.test.state.kv_entries",
				Value:  0,
				Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
			},
			// --- config entries ---
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-intentions": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-intentions"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-intentions": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-intentions"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-resolver": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-resolver"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-resolver": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-resolver"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-router": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-router"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-router": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-router"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-defaults": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-defaults"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-defaults": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-defaults"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=ingress-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=ingress-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "ingress-gateway"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=service-splitter": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-splitter"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=service-splitter": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "service-splitter"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=mesh": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=mesh": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "mesh"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=proxy-defaults": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "proxy-defaults"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=proxy-defaults": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "proxy-defaults"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=terminating-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=terminating-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "terminating-gateway"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=exported-services": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "exported-services"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=exported-services": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "exported-services"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=sameness-group": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "sameness-group"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=sameness-group": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "sameness-group"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=api-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=api-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "api-gateway"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=bound-api-gateway": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "bound-api-gateway"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=bound-api-gateway": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "bound-api-gateway"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=inline-certificate": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "inline-certificate"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=inline-certificate": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "inline-certificate"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=http-route": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "http-route"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=http-route": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "http-route"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=tcp-route": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "tcp-route"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=tcp-route": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "tcp-route"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=jwt-provider": { // Legacy
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "jwt-provider"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=jwt-provider": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "jwt-provider"},
				},
			},
			"consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=control-plane-request-limit": {
				Name:  "consul.usage.test.consul.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "control-plane-request-limit"},
				},
			},
			"consul.usage.test.state.config_entries;datacenter=dc1;kind=control-plane-request-limit": {
				Name:  "consul.usage.test.state.config_entries",
				Value: 0,
				Labels: []metrics.Label{
					{Name: "datacenter", Value: "dc1"},
					{Name: "kind", Value: "control-plane-request-limit"},
				},
			},
		},
	},
}

func TestUsageReporter_emitNodeUsage_OSS(t *testing.T) {
	cases := baseCases

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			// Only have a single interval for the test
			sink := metrics.NewInmemSink(1*time.Minute, 1*time.Minute)
			cfg := metrics.DefaultConfig("consul.usage.test")
			cfg.EnableHostname = false
			metrics.NewGlobal(cfg, sink)

			mockStateProvider := &mockStateProvider{}
			s, err := newStateStore()
			require.NoError(t, err)
			if tcase.modfiyStateStore != nil {
				tcase.modfiyStateStore(t, s)
			}
			mockStateProvider.On("State").Return(s)

			reporter, err := NewUsageMetricsReporter(
				new(Config).
					WithStateProvider(mockStateProvider).
					WithLogger(testutil.Logger(t)).
					WithDatacenter("dc1").
					WithGetMembersFunc(tcase.getMembersFunc),
			)
			require.NoError(t, err)

			reporter.runOnce()

			intervals := sink.Data()
			require.Len(t, intervals, 1)
			intv := intervals[0]

			assertEqualGaugeMaps(t, tcase.expectedGauges, intv.Gauges)
		})
	}
}

func TestUsageReporter_emitPeeringUsage_OSS(t *testing.T) {
	cases := make(map[string]testCase)
	for k, v := range baseCases {
		eg := make(map[string]metrics.GaugeValue)
		for k, v := range v.expectedGauges {
			eg[k] = v
		}
		cases[k] = testCase{v.modfiyStateStore, v.getMembersFunc, eg}
	}
	peeringsCase := cases["nodes"]
	peeringsCase.modfiyStateStore = func(t *testing.T, s *state.Store) {
		id, err := uuid.GenerateUUID()
		require.NoError(t, err)
		require.NoError(t, s.PeeringWrite(1, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{Name: "foo", ID: id}}))
		id, err = uuid.GenerateUUID()
		require.NoError(t, err)
		require.NoError(t, s.PeeringWrite(2, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{Name: "bar", ID: id}}))
		id, err = uuid.GenerateUUID()
		require.NoError(t, err)
		require.NoError(t, s.PeeringWrite(3, &pbpeering.PeeringWriteRequest{Peering: &pbpeering.Peering{Name: "baz", ID: id}}))
	}
	peeringsCase.getMembersFunc = func() []serf.Member {
		return []serf.Member{
			{
				Name:   "foo",
				Tags:   map[string]string{"role": "consul"},
				Status: serf.StatusAlive,
			},
			{
				Name:   "bar",
				Tags:   map[string]string{"role": "consul"},
				Status: serf.StatusAlive,
			},
		}
	}
	peeringsCase.expectedGauges["consul.usage.test.consul.state.nodes;datacenter=dc1"] = metrics.GaugeValue{ // Legacy
		Name:   "consul.usage.test.consul.state.nodes",
		Value:  0,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	peeringsCase.expectedGauges["consul.usage.test.state.nodes;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.nodes",
		Value:  0,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	peeringsCase.expectedGauges["consul.usage.test.consul.state.peerings;datacenter=dc1"] = metrics.GaugeValue{ // Legacy
		Name:   "consul.usage.test.consul.state.peerings",
		Value:  3,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	peeringsCase.expectedGauges["consul.usage.test.state.peerings;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.peerings",
		Value:  3,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	peeringsCase.expectedGauges["consul.usage.test.consul.members.clients;datacenter=dc1"] = metrics.GaugeValue{ // Legacy
		Name:   "consul.usage.test.consul.members.clients",
		Value:  0,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	peeringsCase.expectedGauges["consul.usage.test.members.clients;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.members.clients",
		Value:  0,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	cases["peerings"] = peeringsCase
	delete(cases, "nodes")

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			// Only have a single interval for the test
			sink := metrics.NewInmemSink(1*time.Minute, 1*time.Minute)
			cfg := metrics.DefaultConfig("consul.usage.test")
			cfg.EnableHostname = false
			metrics.NewGlobal(cfg, sink)

			mockStateProvider := &mockStateProvider{}
			s, err := newStateStore()
			require.NoError(t, err)
			if tcase.modfiyStateStore != nil {
				tcase.modfiyStateStore(t, s)
			}
			mockStateProvider.On("State").Return(s)

			reporter, err := NewUsageMetricsReporter(
				new(Config).
					WithStateProvider(mockStateProvider).
					WithLogger(testutil.Logger(t)).
					WithDatacenter("dc1").
					WithGetMembersFunc(tcase.getMembersFunc),
			)
			require.NoError(t, err)

			reporter.runOnce()

			intervals := sink.Data()
			require.Len(t, intervals, 1)
			intv := intervals[0]

			assertEqualGaugeMaps(t, tcase.expectedGauges, intv.Gauges)
		})
	}
}

func TestUsageReporter_emitServiceUsage_OSS(t *testing.T) {
	cases := make(map[string]testCase)
	for k, v := range baseCases {
		eg := make(map[string]metrics.GaugeValue)
		for k, v := range v.expectedGauges {
			eg[k] = v
		}
		cases[k] = testCase{v.modfiyStateStore, v.getMembersFunc, eg}
	}

	nodesAndSvcsCase := cases["nodes"]
	nodesAndSvcsCase.modfiyStateStore = func(t *testing.T, s *state.Store) {
		require.NoError(t, s.EnsureNode(1, &structs.Node{Node: "foo", Address: "127.0.0.1"}))
		require.NoError(t, s.EnsureNode(2, &structs.Node{Node: "bar", Address: "127.0.0.2"}))
		require.NoError(t, s.EnsureNode(3, &structs.Node{Node: "baz", Address: "127.0.0.2"}))
		require.NoError(t, s.EnsureNode(4, &structs.Node{Node: "qux", Address: "127.0.0.3"}))

		apigw := structs.TestNodeServiceAPIGateway(t)
		apigw.ID = "api-gateway"

		mgw := structs.TestNodeServiceMeshGateway(t)
		mgw.ID = "mesh-gateway"

		tgw := structs.TestNodeServiceTerminatingGateway(t, "1.1.1.1")
		tgw.ID = "terminating-gateway"
		// Typical services and some consul services spread across two nodes
		require.NoError(t, s.EnsureService(5, "foo", &structs.NodeService{ID: "db", Service: "db", Tags: nil, Address: "", Port: 5000}))
		require.NoError(t, s.EnsureService(6, "bar", &structs.NodeService{ID: "api", Service: "api", Tags: nil, Address: "", Port: 5000}))
		require.NoError(t, s.EnsureService(7, "foo", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
		require.NoError(t, s.EnsureService(8, "bar", &structs.NodeService{ID: "consul", Service: "consul", Tags: nil}))
		require.NoError(t, s.EnsureService(9, "foo", &structs.NodeService{ID: "db-connect-proxy", Service: "db-connect-proxy", Tags: nil, Address: "", Port: 5000, Kind: structs.ServiceKindConnectProxy}))
		require.NoError(t, s.EnsureRegistration(10, structs.TestRegisterIngressGateway(t)))
		require.NoError(t, s.EnsureService(11, "foo", mgw))
		require.NoError(t, s.EnsureService(12, "foo", tgw))
		require.NoError(t, s.EnsureService(13, "foo", apigw))
		require.NoError(t, s.EnsureService(14, "bar", &structs.NodeService{ID: "db-native", Service: "db", Tags: nil, Address: "", Port: 5000, Connect: structs.ServiceConnect{Native: true}}))
		require.NoError(t, s.EnsureConfigEntry(15, &structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: "foo",
		}))
		require.NoError(t, s.EnsureConfigEntry(16, &structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: "bar",
		}))
		require.NoError(t, s.EnsureConfigEntry(17, &structs.IngressGatewayConfigEntry{
			Kind: structs.IngressGateway,
			Name: "baz",
		}))
	}
	baseCaseMembers := nodesAndSvcsCase.getMembersFunc()
	nodesAndSvcsCase.getMembersFunc = func() []serf.Member {
		baseCaseMembers = append(baseCaseMembers, serf.Member{
			Name:   "baz",
			Tags:   map[string]string{"role": "node", "segment": "a"},
			Status: serf.StatusAlive,
		})
		baseCaseMembers = append(baseCaseMembers, serf.Member{
			Name:   "qux",
			Tags:   map[string]string{"role": "node", "segment": "b"},
			Status: serf.StatusAlive,
		})
		return baseCaseMembers
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.nodes;datacenter=dc1"] = metrics.GaugeValue{ // Legacy
		Name:   "consul.usage.test.consul.state.nodes",
		Value:  4,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.nodes;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.nodes",
		Value:  4,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.members.clients;datacenter=dc1"] = metrics.GaugeValue{ // Legacy
		Name:   "consul.usage.test.consul.members.clients",
		Value:  2,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.members.clients;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.members.clients",
		Value:  2,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.services;datacenter=dc1"] = metrics.GaugeValue{ // Legacy
		Name:   "consul.usage.test.consul.state.services",
		Value:  8,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.services;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.services",
		Value:  8,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.service_instances;datacenter=dc1"] = metrics.GaugeValue{ // Legacy
		Name:   "consul.usage.test.consul.state.service_instances",
		Value:  10,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.service_instances;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.service_instances",
		Value:  10,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=connect-proxy"] = metrics.GaugeValue{ // Legacy
		Name:  "consul.usage.test.consul.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "connect-proxy"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-proxy"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "connect-proxy"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=terminating-gateway"] = metrics.GaugeValue{ // Legacy
		Name:  "consul.usage.test.consul.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "terminating-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=terminating-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "terminating-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=ingress-gateway"] = metrics.GaugeValue{ // Legacy
		Name:  "consul.usage.test.consul.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "ingress-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=ingress-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "ingress-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=api-gateway"] = metrics.GaugeValue{ // Legacy
		Name:  "consul.usage.test.consul.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "api-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=api-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "api-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=mesh-gateway"] = metrics.GaugeValue{ // Legacy
		Name:  "consul.usage.test.consul.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "mesh-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=mesh-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "mesh-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.connect_instances;datacenter=dc1;kind=connect-native"] = metrics.GaugeValue{ // Legacy
		Name:  "consul.usage.test.consul.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "connect-native"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.connect_instances;datacenter=dc1;kind=connect-native"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.connect_instances",
		Value: 1,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "connect-native"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.billable_service_instances;datacenter=dc1"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.billable_service_instances",
		Value: 3,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.consul.state.config_entries;datacenter=dc1;kind=ingress-gateway"] = metrics.GaugeValue{ // Legacy
		Name:  "consul.usage.test.consul.state.config_entries",
		Value: 3,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "ingress-gateway"},
		},
	}
	nodesAndSvcsCase.expectedGauges["consul.usage.test.state.config_entries;datacenter=dc1;kind=ingress-gateway"] = metrics.GaugeValue{
		Name:  "consul.usage.test.state.config_entries",
		Value: 3,
		Labels: []metrics.Label{
			{Name: "datacenter", Value: "dc1"},
			{Name: "kind", Value: "ingress-gateway"},
		},
	}
	cases["nodes-and-services"] = nodesAndSvcsCase
	delete(cases, "nodes")

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			// Only have a single interval for the test
			sink := metrics.NewInmemSink(1*time.Minute, 1*time.Minute)
			cfg := metrics.DefaultConfig("consul.usage.test")
			cfg.EnableHostname = false
			metrics.NewGlobal(cfg, sink)

			mockStateProvider := &mockStateProvider{}
			s := state.NewStateStore(nil)
			if tcase.modfiyStateStore != nil {
				tcase.modfiyStateStore(t, s)
			}
			mockStateProvider.On("State").Return(s)

			reporter, err := NewUsageMetricsReporter(
				new(Config).
					WithStateProvider(mockStateProvider).
					WithLogger(testutil.Logger(t)).
					WithDatacenter("dc1").
					WithGetMembersFunc(tcase.getMembersFunc),
			)
			require.NoError(t, err)

			reporter.runOnce()

			intervals := sink.Data()
			require.Len(t, intervals, 1)
			intv := intervals[0]

			assertEqualGaugeMaps(t, tcase.expectedGauges, intv.Gauges)
		})
	}
}

func TestUsageReporter_emitKVUsage_OSS(t *testing.T) {
	cases := make(map[string]testCase)
	for k, v := range baseCases {
		eg := make(map[string]metrics.GaugeValue)
		for k, v := range v.expectedGauges {
			eg[k] = v
		}
		cases[k] = testCase{v.modfiyStateStore, v.getMembersFunc, eg}
	}

	nodesCase := cases["nodes"]
	mss := nodesCase.modfiyStateStore
	nodesCase.modfiyStateStore = func(t *testing.T, s *state.Store) {
		mss(t, s)
		require.NoError(t, s.KVSSet(4, &structs.DirEntry{Key: "a", Value: []byte{1}}))
		require.NoError(t, s.KVSSet(5, &structs.DirEntry{Key: "b", Value: []byte{1}}))
		require.NoError(t, s.KVSSet(6, &structs.DirEntry{Key: "c", Value: []byte{1}}))
		require.NoError(t, s.KVSSet(7, &structs.DirEntry{Key: "d", Value: []byte{1}}))
		require.NoError(t, s.KVSDelete(8, "d", &acl.EnterpriseMeta{}))
		require.NoError(t, s.KVSDelete(9, "c", &acl.EnterpriseMeta{}))
		require.NoError(t, s.KVSSet(10, &structs.DirEntry{Key: "e", Value: []byte{1}}))
		require.NoError(t, s.KVSSet(11, &structs.DirEntry{Key: "f", Value: []byte{1}}))
	}
	nodesCase.expectedGauges["consul.usage.test.consul.state.kv_entries;datacenter=dc1"] = metrics.GaugeValue{ // Legacy
		Name:   "consul.usage.test.consul.state.kv_entries",
		Value:  4,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	nodesCase.expectedGauges["consul.usage.test.state.kv_entries;datacenter=dc1"] = metrics.GaugeValue{
		Name:   "consul.usage.test.state.kv_entries",
		Value:  4,
		Labels: []metrics.Label{{Name: "datacenter", Value: "dc1"}},
	}
	cases["nodes"] = nodesCase

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			// Only have a single interval for the test
			sink := metrics.NewInmemSink(1*time.Minute, 1*time.Minute)
			cfg := metrics.DefaultConfig("consul.usage.test")
			cfg.EnableHostname = false
			metrics.NewGlobal(cfg, sink)

			mockStateProvider := &mockStateProvider{}
			s, err := newStateStore()
			require.NoError(t, err)
			if tcase.modfiyStateStore != nil {
				tcase.modfiyStateStore(t, s)
			}
			mockStateProvider.On("State").Return(s)

			reporter, err := NewUsageMetricsReporter(
				new(Config).
					WithStateProvider(mockStateProvider).
					WithLogger(testutil.Logger(t)).
					WithDatacenter("dc1").
					WithGetMembersFunc(tcase.getMembersFunc),
			)
			require.NoError(t, err)

			reporter.runOnce()

			intervals := sink.Data()
			require.Len(t, intervals, 1)
			intv := intervals[0]

			assertEqualGaugeMaps(t, tcase.expectedGauges, intv.Gauges)
		})
	}
}
