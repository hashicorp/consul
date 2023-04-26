// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"
)

func TestExportedServicesConfigEntry(t *testing.T) {
	cases := map[string]configEntryTestcase{
		"validate: empty service name": {
			entry: &ExportedServicesConfigEntry{
				Name: "default",
				Services: []ExportedService{
					{
						Name: "",
					},
				},
			},
			validateErr: `service name cannot be empty`,
		},
		"validate: empty consumer list": {
			entry: &ExportedServicesConfigEntry{
				Name: "default",
				Services: []ExportedService{
					{
						Name: "web",
					},
				},
			},
			validateErr: `must have at least one consumer`,
		},
		"validate: no wildcard in consumer partition": {
			entry: &ExportedServicesConfigEntry{
				Name: "default",
				Services: []ExportedService{
					{
						Name: "api",
						Consumers: []ServiceConsumer{
							{
								Partition: "foo",
							},
						},
					},
					{
						Name: "web",
						Consumers: []ServiceConsumer{
							{
								Partition: "*",
							},
						},
					},
				},
			},
			validateErr: `Services[1].Consumers[0]: exporting to all partitions (wildcard) is not supported`,
		},
		"validate: no wildcard in consumer peername": {
			entry: &ExportedServicesConfigEntry{
				Name: "default",
				Services: []ExportedService{
					{
						Name: "web",
						Consumers: []ServiceConsumer{
							{
								Peer: "foo",
							},
							{
								Peer: "*",
							},
						},
					},
				},
			},
			validateErr: `Services[0].Consumers[1]: exporting to all peers (wildcard) is not supported`,
		},
		"validate: cannot specify consumer with partition and peername": {
			entry: &ExportedServicesConfigEntry{
				Name: "default",
				Services: []ExportedService{
					{
						Name: "web",
						Consumers: []ServiceConsumer{
							{
								Partition: "foo",
								Peer:      "bar",
							},
						},
					},
				},
			},
			validateErr: `Services[0].Consumers[0]: must define at most one of Peer, Partition, or SamenessGroup`,
		},
	}

	testConfigEntryNormalizeAndValidate(t, cases)
}
