// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package resource

import "github.com/hashicorp/consul/proto-public/v2/pbresource"

var (
	TypeV1Tombstone = &pbresource.Type{
		Group:        "internal",
		GroupVersion: "v1",
		Kind:         "Tombstone",
	}
)
