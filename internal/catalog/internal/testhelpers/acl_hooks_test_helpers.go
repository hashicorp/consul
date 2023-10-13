// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package testhelpers

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// WorkloadSelecting denotes a resource type that uses workload selectors.
type WorkloadSelecting interface {
	proto.Message
	GetWorkloads() *pbcatalog.WorkloadSelector
}

func RunWorkloadSelectingTypeACLsTests[T WorkloadSelecting](t *testing.T, typ *pbresource.Type,
	getData func(selector *pbcatalog.WorkloadSelector) T,
	registerFunc func(registry resource.Registry),
) {
	// Wire up a registry to generically invoke hooks
	registry := resource.NewRegistry()
	registerFunc(registry)

	cases := map[string]resourcetest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Data:    getData(&pbcatalog.WorkloadSelector{Names: []string{"workload"}}),
			Typ:     typ,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test read": {
			Rules:   `service "test" { policy = "read" }`,
			Data:    getData(&pbcatalog.WorkloadSelector{Names: []string{"workload"}}),
			Typ:     typ,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test write with named selectors and insufficient policy": {
			Rules:   `service "test" { policy = "write" }`,
			Data:    getData(&pbcatalog.WorkloadSelector{Names: []string{"workload"}}),
			Typ:     typ,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test write with prefixed selectors and insufficient policy": {
			Rules:   `service "test" { policy = "write" }`,
			Data:    getData(&pbcatalog.WorkloadSelector{Prefixes: []string{"workload"}}),
			Typ:     typ,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test write with named selectors": {
			Rules:   `service "test" { policy = "write" } service "workload" { policy = "read" }`,
			Data:    getData(&pbcatalog.WorkloadSelector{Names: []string{"workload"}}),
			Typ:     typ,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test write with prefixed selectors": {
			Rules:   `service "test" { policy = "write" } service_prefix "workload-" { policy = "read" }`,
			Data:    getData(&pbcatalog.WorkloadSelector{Prefixes: []string{"workload-"}}),
			Typ:     typ,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test write with prefixed selectors and a policy with more specific than the selector": {
			Rules:   `service "test" { policy = "write" } service_prefix "workload-" { policy = "read" }`,
			Data:    getData(&pbcatalog.WorkloadSelector{Prefixes: []string{"wor"}}),
			Typ:     typ,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test write with prefixed selectors and a policy with less specific than the selector": {
			Rules:   `service "test" { policy = "write" } service_prefix "wor" { policy = "read" }`,
			Data:    getData(&pbcatalog.WorkloadSelector{Prefixes: []string{"workload-"}}),
			Typ:     typ,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test write with prefixed selectors and a policy with a specific service": {
			Rules:  `service "test" { policy = "write" } service "workload" { policy = "read" }`,
			Data:   getData(&pbcatalog.WorkloadSelector{Prefixes: []string{"workload"}}),
			Typ:    typ,
			ReadOK: resourcetest.ALLOW,
			// TODO (ishustava): this is wrong and should be fixed in a follow up PR. We should not allow
			// a policy for a specific service when only prefixes are specified in the selector.
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}
}
