package proxysupport

// EnvoyVersions lists the latest officially supported versions of envoy.
//
// This list must be sorted by semver descending. Only one point release for
// each major release should be present.
//
// see: https://www.consul.io/docs/connect/proxies/envoy#supported-versions
var EnvoyVersions = []string{
	"1.16.4",
	"1.15.5",
	"1.14.6",
	"1.13.7",
}
