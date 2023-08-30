// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfg

import (
	"context"
	"errors"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/leafcert"
	"github.com/hashicorp/consul/agent/structs"
)

// UpdateEvent contains new data for a resource we are subscribed to (e.g. an
// agent cache entry).
type UpdateEvent struct {
	CorrelationID string
	Result        interface{}
	Err           error
}

// TerminalError wraps the given error to indicate that the data source is in
// an irrecoverably broken state (e.g. because the given ACL token has been
// deleted).
//
// Setting UpdateEvent.Err to a TerminalError causes all watches to be canceled
// which, in turn, terminates the xDS streams.
func TerminalError(err error) error {
	return terminalError{err}
}

// IsTerminalError returns whether the given error indicates that the data
// source is in an irrecoverably broken state so watches should be torn down
// and retried at a higher level.
func IsTerminalError(err error) bool {
	return errors.As(err, &terminalError{})
}

type terminalError struct{ err error }

func (e terminalError) Error() string { return e.err.Error() }
func (e terminalError) Unwrap() error { return e.err }

// DataSources contains the dependencies used to consume data used to configure
// proxies.
type DataSources struct {
	// CARoots provides updates about the CA root certificates on a notification
	// channel.
	CARoots CARoots

	// CompiledDiscoveryChain provides updates about a service's discovery chain
	// on a notification channel.
	CompiledDiscoveryChain CompiledDiscoveryChain

	// ConfigEntry provides updates about a single config entry on a notification
	// channel.
	ConfigEntry ConfigEntry

	// ConfigEntryList provides updates about a list of config entries on a
	// notification channel.
	ConfigEntryList ConfigEntryList

	// Datacenters provides updates about federated datacenters on a notification
	// channel.
	Datacenters Datacenters

	// FederationStateListMeshGateways is the interface used to consume updates
	// about mesh gateways from the federation state.
	FederationStateListMeshGateways FederationStateListMeshGateways

	// GatewayServices provides updates about a gateway's upstream services on a
	// notification channel.
	GatewayServices GatewayServices

	// ServiceGateways provides updates about a gateway's upstream services on a
	// notification channel.
	ServiceGateways ServiceGateways

	// Health provides service health updates on a notification channel.
	Health Health

	// HTTPChecks provides updates about a service's HTTP and gRPC checks on a
	// notification channel.
	HTTPChecks HTTPChecks

	// Intentions provides intention updates on a notification channel.
	Intentions Intentions

	// IntentionUpstreams provides intention-inferred upstream updates on a
	// notification channel.
	IntentionUpstreams IntentionUpstreams

	// IntentionUpstreamsDestination provides intention-inferred upstream updates on a
	// notification channel.
	IntentionUpstreamsDestination IntentionUpstreams

	// InternalServiceDump provides updates about services of a given kind (e.g.
	// mesh gateways) on a notification channel.
	InternalServiceDump InternalServiceDump

	// LeafCertificate provides updates about the service's leaf certificate on a
	// notification channel.
	LeafCertificate LeafCertificate

	// PeeredUpstreams provides imported-service upstream updates on a
	// notification channel.
	PeeredUpstreams PeeredUpstreams

	// PeeringList provides peering updates on a notification channel.
	PeeringList PeeringList

	// PreparedQuery provides updates about the results of a prepared query.
	PreparedQuery PreparedQuery

	// ResolvedServiceConfig provides updates about a service's resolved config.
	ResolvedServiceConfig ResolvedServiceConfig

	// ServiceList provides updates about the list of all services in a datacenter
	// on a notification channel.
	ServiceList ServiceList

	// TrustBundle provides updates about the trust bundle for a single peer.
	TrustBundle TrustBundle

	// TrustBundleList provides updates about the list of trust bundles for
	// peered clusters that the given proxy is exported to.
	TrustBundleList TrustBundleList

	// ExportedPeeredServices provides updates about the list of all exported
	// services in a datacenter on a notification channel.
	ExportedPeeredServices ExportedPeeredServices

	DataSourcesEnterprise
}

// CARoots is the interface used to consume updates about the CA root
// certificates.
type CARoots interface {
	Notify(ctx context.Context, req *structs.DCSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}

// CompiledDiscoveryChain is the interface used to consume updates about the
// compiled discovery chain for a service.
type CompiledDiscoveryChain interface {
	Notify(ctx context.Context, req *structs.DiscoveryChainRequest, correlationID string, ch chan<- UpdateEvent) error
}

// ConfigEntry is the interface used to consume updates about a single config
// entry.
type ConfigEntry interface {
	Notify(ctx context.Context, req *structs.ConfigEntryQuery, correlationID string, ch chan<- UpdateEvent) error
}

// ConfigEntryList is the interface used to consume updates about a list of config
// entries.
type ConfigEntryList interface {
	Notify(ctx context.Context, req *structs.ConfigEntryQuery, correlationID string, ch chan<- UpdateEvent) error
}

// Datacenters is the interface used to consume updates about federated
// datacenters.
type Datacenters interface {
	Notify(ctx context.Context, req *structs.DatacentersRequest, correlationID string, ch chan<- UpdateEvent) error
}

// FederationStateListMeshGateways is the interface used to consume updates
// about mesh gateways from the federation state.
type FederationStateListMeshGateways interface {
	Notify(ctx context.Context, req *structs.DCSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}

// GatewayServices is the interface used to consume updates about a gateway's
// upstream services.
type GatewayServices interface {
	Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}

// ServiceGateways is the interface used to consume updates about a service terminating gateways
type ServiceGateways interface {
	Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}

// Health is the interface used to consume service health updates.
type Health interface {
	Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}

// HTTPChecks is the interface used to consume updates about a service's HTTP
// and gRPC-based checks (in order to determine which paths to expose through
// the proxy).
type HTTPChecks interface {
	Notify(ctx context.Context, req *cachetype.ServiceHTTPChecksRequest, correlationID string, ch chan<- UpdateEvent) error
}

// Intentions is the interface used to consume intention updates.
type Intentions interface {
	Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}

// IntentionUpstreams is the interface used to consume updates about upstreams
// inferred from service intentions.
type IntentionUpstreams interface {
	Notify(ctx context.Context, req *structs.ServiceSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}

// InternalServiceDump is the interface used to consume updates about services
// of a given kind (e.g. mesh gateways).
type InternalServiceDump interface {
	Notify(ctx context.Context, req *structs.ServiceDumpRequest, correlationID string, ch chan<- UpdateEvent) error
}

// LeafCertificate is the interface used to consume updates about a service's
// leaf certificate.
type LeafCertificate interface {
	Notify(ctx context.Context, req *leafcert.ConnectCALeafRequest, correlationID string, ch chan<- UpdateEvent) error
}

// PeeredUpstreams is the interface used to consume updates about upstreams
// for all peered targets in a given partition.
type PeeredUpstreams interface {
	Notify(ctx context.Context, req *structs.PartitionSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}

// PeeringList is the interface used to consume updates about peerings in the cluster or partition
type PeeringList interface {
	Notify(ctx context.Context, req *cachetype.PeeringListRequest, correlationID string, ch chan<- UpdateEvent) error
}

// PreparedQuery is the interface used to consume updates about the results of
// a prepared query.
type PreparedQuery interface {
	Notify(ctx context.Context, req *structs.PreparedQueryExecuteRequest, correlationID string, ch chan<- UpdateEvent) error
}

// ResolvedServiceConfig is the interface used to consume updates about a
// service's resolved config.
type ResolvedServiceConfig interface {
	Notify(ctx context.Context, req *structs.ServiceConfigRequest, correlationID string, ch chan<- UpdateEvent) error
}

// ServiceList is the interface used to consume updates about the list of
// all services in a datacenter.
type ServiceList interface {
	Notify(ctx context.Context, req *structs.DCSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}

// TrustBundle is the interface used to consume updates about a single
// peer's trust bundle.
type TrustBundle interface {
	Notify(ctx context.Context, req *cachetype.TrustBundleReadRequest, correlationID string, ch chan<- UpdateEvent) error
}

// TrustBundleList is the interface used to consume updates about trust bundles
// for peered clusters that the given proxy is exported to.
type TrustBundleList interface {
	Notify(ctx context.Context, req *cachetype.TrustBundleListRequest, correlationID string, ch chan<- UpdateEvent) error
}

// ExportedPeeredServices is the interface used to consume updates about the
// list of all services exported to peers in a datacenter.
type ExportedPeeredServices interface {
	Notify(ctx context.Context, req *structs.DCSpecificRequest, correlationID string, ch chan<- UpdateEvent) error
}
