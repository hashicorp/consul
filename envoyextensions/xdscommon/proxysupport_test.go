// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package xdscommon

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/hashicorp/go-version"
)

func TestProxySupportOrder(t *testing.T) {
	versions := make([]*version.Version, len(EnvoyVersions))
	beforeSort := make([]*version.Version, len(EnvoyVersions))
	for i, raw := range EnvoyVersions {
		v, _ := version.NewVersion(raw)
		versions[i] = v
		beforeSort[i] = v
	}

	// After this, the versions are properly sorted
	// go-version has a collection container, but it only allows for sorting in ascending order
	sort.Slice(versions, func(i, j int) bool {
		return versions[j].LessThan(versions[i])
	})

	// Check that we already have a sorted list
	for i := range EnvoyVersions {
		assert.True(t, versions[i].Equal(beforeSort[i]))
	}
}
