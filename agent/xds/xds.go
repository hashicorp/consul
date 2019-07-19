// Package xds provides an impementation of a gRPC service that exports Envoy's
// xDS API for config discovery. Specifically we support the Aggregated
// Discovery Service (ADS) only as we control all config.
//
// A full description of the XDS protocol can be found at
// https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol
//
// xds.Server also support ext_authz network filter API to authorize incoming
// connections to Envoy.
package xds
