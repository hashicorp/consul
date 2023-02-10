package xds

import (
	"errors"
	"fmt"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

// secretsFromSnapshot returns the xDS API representation of the "secrets"
// in the snapshot
func (s *ResourceGenerator) secretsFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy,
		structs.ServiceKindTerminatingGateway,
		structs.ServiceKindMeshGateway,
		structs.ServiceKindIngressGateway:
		return nil, nil
	// Only API gateways utilize secrets
	case structs.ServiceKindAPIGateway:
		return s.secretsFromSnapshotAPIGateway(cfgSnap)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

func (s *ResourceGenerator) secretsFromSnapshotAPIGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	resources := make([]proto.Message, 0, cfgSnap.APIGateway.Certificates.Len())

	cfgSnap.APIGateway.Certificates.ForEachKey(func(ref structs.ResourceReference) bool {
		cert, ok := cfgSnap.APIGateway.Certificates.Get(ref)
		if !ok {
			// inline-certificate not present in map for some reason, process others
			return true
		}

		secret := &envoy_tls_v3.Secret{
			Name: ref.String(),
			Type: &envoy_tls_v3.Secret_TlsCertificate{
				TlsCertificate: &envoy_tls_v3.TlsCertificate{
					CertificateChain: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: lib.EnsureTrailingNewline(cert.Certificate),
						},
					},
					PrivateKey: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: lib.EnsureTrailingNewline(cert.PrivateKey),
						},
					},
				},
			},
		}

		resources = append(resources, secret)
		return true
	})

	return resources, nil
}
