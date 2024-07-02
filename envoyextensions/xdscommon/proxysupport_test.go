// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdscommon

import (
	"slices"
	"sort"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
)

// TestProxySupportOrder tests that the values in EnvoyVersions are valid (X.Y.Z), contiguous by "major" (X.Y) version,
// and sorted in descending order.
func TestProxySupportOrder(t *testing.T) {
	versions := make([]*version.Version, len(EnvoyVersions))
	beforeSort := make([]*version.Version, len(EnvoyVersions))
	for i, raw := range EnvoyVersions {
		v, _ := version.NewVersion(raw)
		if v.Segments()[0] != 1 {
			// If this fails, we need to add support for a new semver-major (x in x.y.z) version of Envoy
			t.Fatalf("Expected major version to be 1, got: %v", v.Segments()[0])
		}
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

	// Check that we have a continues set of versions
	for i := 1; i < len(versions); i++ {
		previousMajorVersion := getMajorVersion(versions[i-1])
		majorVersion := getMajorVersion(versions[i])
		assert.True(t, majorVersion == previousMajorVersion-1,
			"Expected Envoy major version following %d.%d to be %d.%d, got %d.%d",
			versions[i-1].Segments()[0],
			previousMajorVersion,
			versions[i-1].Segments()[0],
			previousMajorVersion-1,
			versions[i].Segments()[0],
			majorVersion)
	}
}

func TestParseEnvoyVersions(t *testing.T) {
	// Test with valid versions, comments, and blank lines
	raw := "# Comment\n1.29.4\n\n# More\n# comments\n1.28.3\n\n1.27.5\n1.26.8\n\n"
	expected := []string{"1.29.4", "1.28.3", "1.27.5", "1.26.8"}

	versions, err := parseEnvoyVersions(raw)
	assert.NoError(t, err)

	if !slices.Equal(versions, expected) {
		t.Fatalf("Expected %v, got: %v", expected, versions)
	}

	// Test with invalid version
	raw = "1.29.4\n1.26.8\nfoo"

	_, err = parseEnvoyVersions(raw)
	assert.EqualError(t, err, "invalid version in ENVOY_VERSIONS: foo")

	// Test with out-of-order values
	raw = "1.29.4\n1.26.8\n1.27.5"

	_, err = parseEnvoyVersions(raw)
	assert.EqualError(t, err, "ENVOY_VERSIONS must be sorted in descending order")
}

// getMajorVersion returns the "major" (Y in X.Y.Z) version of the given Envoy version.
func getMajorVersion(version *version.Version) int {
	return version.Segments()[1]
}
