// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xdscommon

import (
	"fmt"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/hashicorp/go-version"
)

var (
	// minSupportedVersion is the oldest mainline version we support. This should always be
	// the zero'th point release of the last element of xdscommon.EnvoyVersions.
	minSupportedVersion = version.Must(version.NewVersion(GetMinEnvoyMinorVersion()))

	specificUnsupportedVersions = []unsupportedVersion{}
)

type unsupportedVersion struct {
	Version   *version.Version
	UpgradeTo string
	Why       string
}

type SupportedProxyFeatures struct {
	// Put feature switches here when necessary. For reference, The most recent remove of a feature flag was removed in
	// <insert PR here>.
}

func DetermineSupportedProxyFeatures(node *envoy_core_v3.Node) (SupportedProxyFeatures, error) {
	version := DetermineEnvoyVersionFromNode(node)
	return determineSupportedProxyFeaturesFromVersion(version)
}

func DetermineSupportedProxyFeaturesFromString(vs string) (SupportedProxyFeatures, error) {
	version := version.Must(version.NewVersion(vs))
	return determineSupportedProxyFeaturesFromVersion(version)
}

func determineSupportedProxyFeaturesFromVersion(version *version.Version) (SupportedProxyFeatures, error) {
	if version == nil {
		// This would happen on either extremely old builds OR perhaps on
		// custom builds. Should we error?
		return SupportedProxyFeatures{}, nil
	}

	if version.LessThan(minSupportedVersion) {
		return SupportedProxyFeatures{}, fmt.Errorf("Envoy %s is too old and is not supported by Consul", version)
	}

	for _, uv := range specificUnsupportedVersions {
		if version.Equal(uv.Version) {
			return SupportedProxyFeatures{}, fmt.Errorf(
				"Envoy %s is too old of a point release and is not supported by Consul because it %s. "+
					"Please upgrade to version %s.",
				version,
				uv.Why,
				uv.UpgradeTo,
			)
		}
	}

	sf := SupportedProxyFeatures{}

	// when feature flags necessary, populate here by calling version.LessThan(...)

	return sf, nil
}

func DetermineEnvoyVersionFromNode(node *envoy_core_v3.Node) *version.Version {
	if node == nil {
		return nil
	}

	if node.UserAgentVersionType == nil {
		return nil
	}

	if node.UserAgentName != "envoy" {
		return nil
	}

	bv, ok := node.UserAgentVersionType.(*envoy_core_v3.Node_UserAgentBuildVersion)
	if !ok {
		// NOTE: we could sniff for *envoycore.Node_UserAgentVersion and do more regex but official builds don't have this problem.
		return nil
	}
	if bv.UserAgentBuildVersion == nil {
		return nil
	}
	v := bv.UserAgentBuildVersion.Version

	return version.Must(version.NewVersion(
		fmt.Sprintf("%d.%d.%d",
			v.GetMajorNumber(),
			v.GetMinorNumber(),
			v.GetPatch(),
		),
	))
}
