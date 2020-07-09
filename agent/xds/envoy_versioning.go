package xds

import (
	"fmt"
	"regexp"

	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/hashicorp/go-version"
)

var (
	// minSafeRegexVersion reflects the minimum version where we could use safe_regex instead of regex
	//
	// NOTE: the first version that no longer supported the old style was 1.13.0
	minSafeRegexVersion = version.Must(version.NewVersion("1.11.2"))
)

type supportedProxyFeatures struct {
	RouterMatchSafeRegex bool // use safe_regex instead of regex in http.router rules
}

func determineSupportedProxyFeatures(node *envoycore.Node) supportedProxyFeatures {
	version := determineEnvoyVersionFromNode(node)
	if version == nil {
		return supportedProxyFeatures{}
	}

	return supportedProxyFeatures{
		RouterMatchSafeRegex: !version.LessThan(minSafeRegexVersion),
	}
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

func determineSupportedProxyFeaturesFromString(vs string) supportedProxyFeatures {
	version := version.Must(version.NewVersion(vs))
	return supportedProxyFeatures{
		RouterMatchSafeRegex: !version.LessThan(minSafeRegexVersion),
	}
}
