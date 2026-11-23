// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"testing"

	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestAPIGatewayFrontendValidationStrict(t *testing.T) {
	ref := structs.ResourceReference{
		Kind:           structs.FileSystemCertificate,
		Name:           "client-ca",
		EnterpriseMeta: acl.NewEnterpriseMetaWithPartition("prod", "payments"),
	}
	context := &envoy_tls_v3.DownstreamTlsContext{
		CommonTlsContext: &envoy_tls_v3.CommonTlsContext{},
	}
	validation := &structs.APIGatewayFrontendTLSValidation{
		Mode:              structs.APIGatewayFrontendValidationAllowValidOnly,
		CACertificateRefs: []structs.ResourceReference{ref},
	}

	require.NoError(t, applyAPIGatewayFrontendValidation(context, validation))
	require.True(t, context.RequireClientCertificate.Value)
	require.True(t, context.GetDisableStatelessSessionResumption())
	require.True(t, context.GetDisableStatefulSessionResumption())
	require.Equal(t,
		fmt.Sprintf("gateway-client-ca/%s/%s/client-ca", ref.PartitionOrDefault(), ref.NamespaceOrDefault()),
		context.CommonTlsContext.GetValidationContextSdsSecretConfig().Name,
	)
}

func TestAPIGatewayFrontendValidationAbsentPreservesSessionResumption(t *testing.T) {
	context := &envoy_tls_v3.DownstreamTlsContext{
		CommonTlsContext: &envoy_tls_v3.CommonTlsContext{},
	}
	require.NoError(t, applyAPIGatewayFrontendValidation(context, nil))
	require.Nil(t, context.SessionTicketKeysType)
	require.False(t, context.GetDisableStatefulSessionResumption())
}

func TestAPIGatewayFrontendValidationRejectsUnsafeFallback(t *testing.T) {
	err := applyAPIGatewayFrontendValidation(
		&envoy_tls_v3.DownstreamTlsContext{CommonTlsContext: &envoy_tls_v3.CommonTlsContext{}},
		&structs.APIGatewayFrontendTLSValidation{
			Mode: structs.APIGatewayFrontendValidationAllowInsecureFallback,
			CACertificateRefs: []structs.ResourceReference{{
				Kind: structs.FileSystemCertificate,
				Name: "client-ca",
			}},
		},
	)
	require.ErrorContains(t, err, "not safely supported")
}

func TestAPIGatewayClientCASecret(t *testing.T) {
	ref := structs.ResourceReference{
		Kind:           structs.FileSystemCertificate,
		Name:           "client-ca",
		EnterpriseMeta: acl.NewEnterpriseMetaWithPartition("prod", "payments"),
	}
	snapshot := proxycfg.TestConfigSnapshotAPIGateway(
		t,
		"default",
		nil,
		func(entry *structs.APIGatewayConfigEntry, _ *structs.BoundAPIGatewayConfigEntry) {
			entry.FrontendTLS = &structs.APIGatewayFrontendTLSConfig{
				Default: &structs.APIGatewayFrontendTLSValidation{
					Mode:              structs.APIGatewayFrontendValidationAllowValidOnly,
					CACertificateRefs: []structs.ResourceReference{ref},
				},
			}
		},
		nil,
		nil,
		[]proxycfg.UpdateEvent{{
			CorrelationID: "client-ca-certificate-config:" + ref.String(),
			Result: &structs.ConfigEntryResponse{Entry: &structs.FileSystemCertificateConfigEntry{
				Name:           ref.Name,
				Certificate:    "/consul/gateway-certificates/client-ca.crt",
				EnterpriseMeta: ref.EnterpriseMeta,
			}},
		}},
	)

	resources := (&ResourceGenerator{}).secretsFromSnapshotAPIGateway(snapshot)
	require.Len(t, resources, 1)
	secret := resources[0].(*envoy_tls_v3.Secret)
	require.Equal(t,
		fmt.Sprintf("gateway-client-ca/%s/%s/client-ca", ref.PartitionOrDefault(), ref.NamespaceOrDefault()),
		secret.Name,
	)
	require.Equal(t,
		"/consul/gateway-certificates/client-ca.crt",
		secret.GetValidationContext().GetTrustedCa().GetFilename(),
	)
	require.Nil(t, secret.GetTlsCertificate())

	require.True(t, apiGatewayFrontendValidationReady(snapshot, 443))
	snapshot.APIGateway.ClientCACertificates.Clear(ref)
	require.False(t, apiGatewayFrontendValidationReady(snapshot, 443))
	snapshot.APIGateway.GatewayConfig.FrontendTLS.Default = &structs.APIGatewayFrontendTLSValidation{
		Mode: structs.APIGatewayFrontendValidationDenyAll,
	}
	require.False(t, apiGatewayFrontendValidationReady(snapshot, 443))
	snapshot.APIGateway.GatewayConfig.FrontendTLS = nil
	require.True(t, apiGatewayFrontendValidationReady(snapshot, 443))
}
