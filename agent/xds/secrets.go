// Copyright (c) HashiCorp, Inc.
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
		structs.ServiceKindMeshGateway,
		structs.ServiceKindIngressGateway:
		return nil, nil
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
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
