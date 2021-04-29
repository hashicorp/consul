package xds

import (
	"fmt"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"

	"github.com/hashicorp/go-version"
)

var (
	// minSupportedVersion is the oldest mainline version we support. This should always be
	// the zero'th point release of the last element of proxysupport.EnvoyVersions.
	minSupportedVersion = version.Must(version.NewVersion("1.15.0"))

	minVersionAllowingEmptyGatewayClustersWithIncrementalXDS = version.Must(version.NewVersion("1.16.0"))
	minVersionAllowingMultipleIncrementalXDSChanges          = version.Must(version.NewVersion("1.16.0"))

	specificUnsupportedVersions = []unsupportedVersion{}
)

type unsupportedVersion struct {
	Version   *version.Version
	UpgradeTo string
	Why       string
}

type supportedProxyFeatures struct {
	// add version dependent feature flags here

	// GatewaysNeedStubClusterWhenEmptyWithIncrementalXDS is needed to paper
	// over some weird envoy behavior.
	//
	// For some reason Envoy versions prior to 1.16.0 when sent an empty CDS
	// list via the incremental xDS protocol will correctly ack the message and
	// just never request LDS resources.
	GatewaysNeedStubClusterWhenEmptyWithIncrementalXDS bool

	// IncrementalXDSUpdatesMustBeSerial is needed to avoid an envoy crash.
	//
	// Versions of Envoy prior to 1.16.0 could crash if multiple in-flight
	// changes to resources were happening during incremental xDS. To prevent
	// that we force serial updates on those older versions.
	//
	// issue: https://github.com/envoyproxy/envoy/issues/11877
	// PR:    https://github.com/envoyproxy/envoy/pull/12069
	IncrementalXDSUpdatesMustBeSerial bool
}

func determineSupportedProxyFeatures(node *envoy_core_v3.Node) (supportedProxyFeatures, error) {
	version := determineEnvoyVersionFromNode(node)
	return determineSupportedProxyFeaturesFromVersion(version)
}

func determineSupportedProxyFeaturesFromString(vs string) (supportedProxyFeatures, error) {
	version := version.Must(version.NewVersion(vs))
	return determineSupportedProxyFeaturesFromVersion(version)
}

func determineSupportedProxyFeaturesFromVersion(version *version.Version) (supportedProxyFeatures, error) {
	if version == nil {
		// This would happen on either extremely old builds OR perhaps on
		// custom builds. Should we error?
		return supportedProxyFeatures{}, nil
	}

	if version.LessThan(minSupportedVersion) {
		return supportedProxyFeatures{}, fmt.Errorf("Envoy %s is too old and is not supported by Consul", version)
	}

	for _, uv := range specificUnsupportedVersions {
		if version.Equal(uv.Version) {
			return supportedProxyFeatures{}, fmt.Errorf(
				"Envoy %s is too old of a point release and is not supported by Consul because it %s. "+
					"Please upgrade to version %s.",
				version,
				uv.Why,
				uv.UpgradeTo,
			)
		}
	}

	sf := supportedProxyFeatures{}

	if version.LessThan(minVersionAllowingEmptyGatewayClustersWithIncrementalXDS) {
		sf.GatewaysNeedStubClusterWhenEmptyWithIncrementalXDS = true
	}

	if version.LessThan(minVersionAllowingMultipleIncrementalXDSChanges) {
		sf.IncrementalXDSUpdatesMustBeSerial = true
	}

	return sf, nil
}

func determineEnvoyVersionFromNode(node *envoy_core_v3.Node) *version.Version {
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
