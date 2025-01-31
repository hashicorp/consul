// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package metadata

import (
	"sync"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
)

type versionTuple struct {
	Value *version.Version
	Err   error
}

var versionCache sync.Map // string->versionTuple

// Build extracts the Consul version info for a member.
func Build(m *serf.Member) (*version.Version, error) {
	build := m.Tags["build"]

	ok, v, err := getMemoizedBuildVersion(build)
	if ok {
		return v, err
	}

	v, err = parseBuildAsVersion(build)

	versionCache.Store(build, versionTuple{Value: v, Err: err})

	return v, err
}

func getMemoizedBuildVersion(build string) (bool, *version.Version, error) {
	rawTuple, ok := versionCache.Load(build)
	if !ok {
		return false, nil, nil
	}
	tuple, ok := rawTuple.(versionTuple)
	if !ok {
		return false, nil, nil
	}
	return true, tuple.Value, tuple.Err
}

func parseBuildAsVersion(build string) (*version.Version, error) {
	str := versionFormat.FindString(build)
	return version.NewVersion(str)
}
