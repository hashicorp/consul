// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package state

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func testIndexerTableCoordinates() map[string]indexerTestCase {
	return map[string]indexerTestCase{
		indexID: {
			read: indexValue{
				source: CoordinateQuery{
					Node:    "NoDeId",
					Segment: "SeGmEnT",
				},
				expected: []byte("nodeid\x00segment\x00"),
			},
			write: indexValue{
				source: &structs.Coordinate{
					Node:    "NoDeId",
					Segment: "SeGmEnT",
				},
				expected: []byte("nodeid\x00segment\x00"),
			},
			prefix: []indexValue{
				{
					source:   (*acl.EnterpriseMeta)(nil),
					expected: nil,
				},
				{
					source:   acl.EnterpriseMeta{},
					expected: nil,
				},
				{
					source:   Query{Value: "NoDeId"},
					expected: []byte("nodeid\x00"),
				},
			},
		},
		indexNode: {
			read: indexValue{
				source: Query{
					Value: "NoDeId",
				},
				expected: []byte("nodeid\x00"),
			},
			write: indexValue{
				source: &structs.Coordinate{
					Node: "NoDeId",
				},
				expected: []byte("nodeid\x00"),
			},
		},
	}
}
