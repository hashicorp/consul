package xds

import (
	"fmt"
	"regexp"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/hashicorp/go-version"
)

var (
	// minSupportedVersion is the oldest mainline version we support. This should always be
	// the zero'th point release of the last element of proxysupport.EnvoyVersions.
	minSupportedVersion = version.Must(version.NewVersion("1.12.0"))

	specificUnsupportedVersions = []unsupportedVersion{
		{
			Version:   version.Must(version.NewVersion("1.12.0")),
			UpgradeTo: "1.12.3+",
			Why:       "does not support RBAC rules using url_path",
		},
		{
			Version:   version.Must(version.NewVersion("1.12.1")),
			UpgradeTo: "1.12.3+",
			Why:       "does not support RBAC rules using url_path",
		},
		{
			Version:   version.Must(version.NewVersion("1.12.2")),
			UpgradeTo: "1.12.3+",
			Why:       "does not support RBAC rules using url_path",
		},
		{
			Version:   version.Must(version.NewVersion("1.13.0")),
			UpgradeTo: "1.13.1+",
			Why:       "does not support RBAC rules using url_path",
		},
	}
)

type unsupportedVersion struct {
	Version   *version.Version
	UpgradeTo string
	Why       string
}

type supportedProxyFeatures struct {
	// add version dependent feature flags here
}

func determineSupportedProxyFeatures(node *envoycore.Node) (supportedProxyFeatures, error) {
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

	return supportedProxyFeatures{}, nil
}

// example: 1580db37e9a97c37e410bad0e1507ae1a0fd9e77/1.12.4/Clean/RELEASE/BoringSSL
var buildVersionPattern = regexp.MustCompile(`^[a-f0-9]{40}/([^/]+)/Clean/RELEASE/BoringSSL$`)

func determineEnvoyVersionFromNode(node *envoycore.Node) *version.Version {
	if node == nil {
		return nil
	}

	if node.UserAgentVersionType == nil {
		if node.BuildVersion == "" {
			return nil
		}

		// Must be an older pre-1.13 envoy
		m := buildVersionPattern.FindStringSubmatch(node.BuildVersion)
		if m == nil {
			return nil
		}

		return version.Must(version.NewVersion(m[1]))
	}

	if node.UserAgentName != "envoy" {
		return nil
	}

	bv, ok := node.UserAgentVersionType.(*envoycore.Node_UserAgentBuildVersion)
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
