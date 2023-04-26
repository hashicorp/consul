// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// EqualStatus compares two status maps for equality.
func EqualStatus(a, b map[string]*pbresource.Status) bool {
	if len(a) != len(b) {
		return false
	}

	compared := make(map[string]struct{})
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !proto.Equal(av, bv) {
			return false
		}
		compared[k] = struct{}{}
	}

	for k, bv := range b {
		if _, skip := compared[k]; skip {
			continue
		}

		av, ok := a[k]
		if !ok {
			return false
		}

		if !proto.Equal(av, bv) {
			return false
		}
	}

	return true
}
