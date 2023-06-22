package proxysupport

// EnvoyVersions lists the latest officially supported versions of envoy.
//
// This list must be sorted by semver descending. Only one point release for
// each major release should be present.
//
// see: https://www.consul.io/docs/connect/proxies/envoy#supported-versions
var EnvoyVersions = []string{
	"1.24.7",
	"1.23.9",
	"1.22.11",
	"1.21.6",
}
