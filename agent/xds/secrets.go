// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

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
	case structs.ServiceKindAPIGateway:
		return s.secretsFromSnapshotAPIGateway(cfgSnap), nil
		// return any attached certs case :
	case structs.ServiceKindTerminatingGateway:
		return s.secretsFromSnapshotTerminatingGateway(cfgSnap), nil
	case structs.ServiceKindConnectProxy,
		structs.ServiceKindMeshGateway:
		return s.secretsFromSnapshotConnectProxy(cfgSnap), nil
	case structs.ServiceKindIngressGateway:
		return nil, nil
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

const (
	connectLeafSecretName = "connect-leaf"
	connectRootSecretName = "connect-root"
)

// secretsFromSnapshotConnectProxy returns the leaf certificate and CA root
// secrets that the public listener's TLS context refers to by name.
//
// Each secret is emitted on its own guard rather than as an all-or-nothing
// pair, matching the terminating gateway behaviour where a CA-only service
// yields a validation context with no tls_certificate. The two halves have
// genuinely independent lifetimes: a mesh gateway with no exported services
// holds valid CA roots but has its leaf watch cancelled and Leaf set to nil
// (see agent/proxycfg/mesh_gateway.go), and during a CA rotation the new roots
// must reach Envoy even if the re-signed leaf has not landed in the snapshot
// yet, otherwise inbound connections presenting a cert from the new CA fail
// validation.
//
// The guards themselves are still required: reading the leaf when it is nil
// would panic, and a validation context whose trusted CA is an empty string is
// rejected by Envoy.
//
// Note that the public listener is generated regardless of what is emitted
// here, because it only ever refers to these secrets by name. That asymmetry
// is deliberate:
//
//   - During a transient gap the listener stays byte-identical, so its filter
//     chain hash does not change, Envoy keeps serving with the secrets it
//     already holds, and no connections are drained. Gating the listener on
//     material readiness instead would rewrite the listener and drain every
//     established connection, which is the exact failure this SDS change
//     exists to remove.
//
//   - On a cold start the secrets never arrive, so the listener stays in
//     warming and never accepts traffic. That fails closed, which is the
//     correct outcome when there is no material to authenticate peers with.
func (s *ResourceGenerator) secretsFromSnapshotConnectProxy(cfgSnap *proxycfg.ConfigSnapshot) []proto.Message {
	var resources []proto.Message

	if leaf := cfgSnap.Leaf(); leaf != nil {
		resources = append(resources, &envoy_tls_v3.Secret{
			Name: connectLeafSecretName,
			Type: &envoy_tls_v3.Secret_TlsCertificate{
				TlsCertificate: &envoy_tls_v3.TlsCertificate{
					CertificateChain: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: lib.EnsureTrailingNewline(leaf.CertPEM),
						},
					},
					PrivateKey: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: lib.EnsureTrailingNewline(leaf.PrivateKeyPEM),
						},
					},
				},
			},
		})
	}

	if rootPEMs := cfgSnap.RootPEMs(); rootPEMs != "" {
		resources = append(resources, &envoy_tls_v3.Secret{
			Name: connectRootSecretName,
			Type: &envoy_tls_v3.Secret_ValidationContext{
				ValidationContext: &envoy_tls_v3.CertificateValidationContext{
					TrustedCa: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_InlineString{
							InlineString: rootPEMs,
						},
					},
				},
			},
		})
	}

	return resources
}

// secretsFromSnapshotAPIGateway returns the "secrets" for an api-gateway service
func (s *ResourceGenerator) secretsFromSnapshotAPIGateway(cfgSnap *proxycfg.ConfigSnapshot) []proto.Message {
	var resources []proto.Message

	cfgSnap.APIGateway.FileSystemCertificates.ForEachKey(func(ref structs.ResourceReference) bool {
		cert, ok := cfgSnap.APIGateway.FileSystemCertificates.Get(ref)
		if !ok || cert == nil {
			return true
		}
		resources = append(resources, &envoy_tls_v3.Secret{
			Name: ref.Name,
			Type: &envoy_tls_v3.Secret_TlsCertificate{
				TlsCertificate: &envoy_tls_v3.TlsCertificate{
					CertificateChain: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_Filename{
							Filename: cert.Certificate,
						}},
					PrivateKey: &envoy_core_v3.DataSource{
						Specifier: &envoy_core_v3.DataSource_Filename{
							Filename: cert.PrivateKey,
						},
					},
				},
			},
		})
		return true
	})

	return resources
}

func (s *ResourceGenerator) secretsFromSnapshotTerminatingGateway(cfgSnap *proxycfg.ConfigSnapshot) []proto.Message {
	var resources []proto.Message
	for _, detail := range cfgSnap.TerminatingGateway.GatewayServices {
		if detail.CertFile != "" && detail.KeyFile != "" {
			resources = append(resources, &envoy_tls_v3.Secret{
				// We use the destination Service Name as the SDS resource "handle"
				Name: detail.Service.Name + "-cert",
				Type: &envoy_tls_v3.Secret_TlsCertificate{
					TlsCertificate: &envoy_tls_v3.TlsCertificate{
						CertificateChain: &envoy_core_v3.DataSource{
							Specifier: &envoy_core_v3.DataSource_Filename{Filename: detail.CertFile},
						},
						PrivateKey: &envoy_core_v3.DataSource{
							Specifier: &envoy_core_v3.DataSource_Filename{Filename: detail.KeyFile},
						},
					},
				},
			})
		}
		if detail.CAFile != "" {
			resources = append(resources, &envoy_tls_v3.Secret{
				Name: detail.Service.Name + "-ca",
				Type: &envoy_tls_v3.Secret_ValidationContext{
					ValidationContext: &envoy_tls_v3.CertificateValidationContext{
						TrustedCa: &envoy_core_v3.DataSource{
							Specifier: &envoy_core_v3.DataSource_Filename{Filename: detail.CAFile},
						},
					},
				},
			})
		}
	}

	return resources
}
