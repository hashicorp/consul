// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package structs

import (
	"testing"
)

func TestExportedServicesConfigEntry_OSS(t *testing.T) {
	cases := map[string]configEntryTestcase{
		"normalize: noop in oss": {
			entry: &ExportedServicesConfigEntry{
				Name: "default",
				Services: []ExportedService{
					{
						Name: "web",
						Consumers: []ServiceConsumer{
							{
								Peer: "bar",
							},
						},
					},
				},
			},
			expected: &ExportedServicesConfigEntry{
				Name: "default",
				Services: []ExportedService{
					{
						Name:      "web",
						Namespace: "",
						Consumers: []ServiceConsumer{
							{
								Peer: "bar",
							},
						},
					},
				},
			},
		},
		"validate: empty name": {
			entry: &ExportedServicesConfigEntry{
				Name: "",
			},
			validateErr: `exported-services Name must be "default"`,
		},
		"validate: wildcard name": {
			entry: &ExportedServicesConfigEntry{
				Name: WildcardSpecifier,
			},
			validateErr: `exported-services Name must be "default"`,
		},
		"validate: other name": {
			entry: &ExportedServicesConfigEntry{
				Name: "foo",
			},
			validateErr: `exported-services Name must be "default"`,
		},
		"validate: sameness groups are enterprise only": {
			entry: &ExportedServicesConfigEntry{
				Name: "default",
				Services: []ExportedService{
					{
						Name: "web",
						Consumers: []ServiceConsumer{
							{
								SamenessGroup: "sg",
							},
						},
					},
				},
			},
			validateErr: `Services[0].Consumers[0]: sameness-groups are an enterprise-only feature`,
		},
	}

	testConfigEntryNormalizeAndValidate(t, cases)
}
