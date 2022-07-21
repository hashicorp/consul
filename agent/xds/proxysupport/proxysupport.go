package proxysupport

// EnvoyVersions lists the latest officially supported versions of envoy.
//
// This list must be sorted by semver descending. Only one point release for
// each major release should be present.
//
// see: https://www.consul.io/docs/connect/proxies/envoy#supported-versions
var EnvoyVersions = []string{
	"1.23.0",
	"1.22.2",
	"1.21.4",
	"1.20.6",
}
