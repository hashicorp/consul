// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package metadata

import (
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
)

// Build extracts the Consul version info for a member.
func Build(m *serf.Member) (*version.Version, error) {
	str := versionFormat.FindString(m.Tags["build"])
	return version.NewVersion(str)
}
