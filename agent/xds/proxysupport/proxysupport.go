package proxysupport

// EnvoyVersions lists the latest officially supported versions of envoy.
//
// This list must be sorted by semver descending. Only one point release for
// each major release should be present.
//
// see: https://www.consul.io/docs/connect/proxies/envoy#supported-versions
var EnvoyVersions = []string{
	// TODO(rb): add in 1.17.0 when the v3 support comes
	"1.16.2",
	"1.15.3",
	"1.14.6",
}
