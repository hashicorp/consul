package proxysupport

// EnvoyVersions lists the latest officially supported versions of envoy.
//
// This list must be sorted by semver descending. Only one point release for
// each major release should be present.
//
// see: https://www.consul.io/docs/connect/proxies/envoy#supported-versions
var EnvoyVersions = []string{
	"1.19.1",
	"1.18.4",
	"1.17.4",
	"1.16.5",
}

var EnvoyVersionsV2 = []string{
	"1.16.5",
}
