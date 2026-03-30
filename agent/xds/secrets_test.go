// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
)

func TestSecretsFromSnapshotTerminatingGateway_NilSnapshot(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	_, err := s.secretsFromSnapshot(nil)
	require.Error(t, err)
}

func TestSecretsFromSnapshotTerminatingGateway_NoServices(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, false, nil, nil)

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Empty(t, resources)
}

func TestSecretsFromSnapshotTerminatingGateway_ServiceWithNoCerts(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	// clear out certs so only services with no cert/key/ca remain
	dbSvc := structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		dbSvc: {
			Service: dbSvc,
			// no CAFile, CertFile, KeyFile
		},
	}

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Empty(t, resources)
}

func TestSecretsFromSnapshotTerminatingGateway_ServiceWithCAOnly(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	svc := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {
			Service: svc,
			CAFile:  "ca.cert.pem",
		},
	}

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Len(t, resources, 1)

	secret, ok := resources[0].(*envoy_tls_v3.Secret)
	require.True(t, ok)
	require.Equal(t, "web-ca", secret.Name)

	vc, ok := secret.Type.(*envoy_tls_v3.Secret_ValidationContext)
	require.True(t, ok)
	require.Equal(t, "ca.cert.pem", vc.ValidationContext.TrustedCa.GetFilename())
}

func TestSecretsFromSnapshotTerminatingGateway_ServiceWithCertAndKeyOnly(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	svc := structs.NewServiceName("api", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {
			Service:  svc,
			CertFile: "api.cert.pem",
			KeyFile:  "api.key.pem",
		},
	}

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Len(t, resources, 1)

	secret, ok := resources[0].(*envoy_tls_v3.Secret)
	require.True(t, ok)
	require.Equal(t, "api-cert", secret.Name)

	tlsCert, ok := secret.Type.(*envoy_tls_v3.Secret_TlsCertificate)
	require.True(t, ok)
	require.Equal(t, "api.cert.pem", tlsCert.TlsCertificate.CertificateChain.GetFilename())
	require.Equal(t, "api.key.pem", tlsCert.TlsCertificate.PrivateKey.GetFilename())
}

func TestSecretsFromSnapshotTerminatingGateway_ServiceWithAllCerts(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	svc := structs.NewServiceName("api", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {
			Service:  svc,
			CAFile:   "ca.cert.pem",
			CertFile: "api.cert.pem",
			KeyFile:  "api.key.pem",
		},
	}

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Len(t, resources, 2)

	secretNames := make(map[string]*envoy_tls_v3.Secret, 2)
	for _, r := range resources {
		sec := r.(*envoy_tls_v3.Secret)
		secretNames[sec.Name] = sec
	}

	certSecret, ok := secretNames["api-cert"]
	require.True(t, ok)
	tlsCert, ok := certSecret.Type.(*envoy_tls_v3.Secret_TlsCertificate)
	require.True(t, ok)
	require.Equal(t, "api.cert.pem", tlsCert.TlsCertificate.CertificateChain.GetFilename())
	require.Equal(t, "api.key.pem", tlsCert.TlsCertificate.PrivateKey.GetFilename())

	caSecret, ok := secretNames["api-ca"]
	require.True(t, ok)
	vc, ok := caSecret.Type.(*envoy_tls_v3.Secret_ValidationContext)
	require.True(t, ok)
	require.Equal(t, "ca.cert.pem", vc.ValidationContext.TrustedCa.GetFilename())
}

func TestSecretsFromSnapshotTerminatingGateway_MultipleServices(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	webSvc := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
	apiSvc := structs.NewServiceName("api", structs.DefaultEnterpriseMetaInDefaultPartition())
	dbSvc := structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition())

	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		webSvc: {
			Service: webSvc,
			CAFile:  "web-ca.pem",
		},
		apiSvc: {
			Service:  apiSvc,
			CAFile:   "api-ca.pem",
			CertFile: "api-cert.pem",
			KeyFile:  "api-key.pem",
		},
		dbSvc: {
			Service: dbSvc,
			// no certs
		},
	}

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	// web contributes 1 (ca), api contributes 2 (cert+ca), db contributes 0
	require.Len(t, resources, 3)

	names := make(map[string]struct{}, len(resources))
	for _, r := range resources {
		sec := r.(*envoy_tls_v3.Secret)
		names[sec.Name] = struct{}{}
	}
	require.Contains(t, names, "web-ca")
	require.Contains(t, names, "api-cert")
	require.Contains(t, names, "api-ca")
}

func TestSecretsFromSnapshotTerminatingGateway_CertFileWithoutKeyFileProducesNoSecret(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	svc := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {
			Service:  svc,
			CertFile: "web-cert.pem",
			// no KeyFile
		},
	}

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Empty(t, resources)
}

func TestSecretsFromSnapshotTerminatingGateway_KeyFileWithoutCertFileProducesNoSecret(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	svc := structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {
			Service: svc,
			KeyFile: "web-key.pem",
			// no CertFile
		},
	}

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Empty(t, resources)
}

func TestSecretsFromSnapshotTerminatingGateway_SecretNamesUsesServiceName(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	svc := structs.NewServiceName("my-special-service", structs.DefaultEnterpriseMetaInDefaultPartition())
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.TerminatingGateway.GatewayServices = map[structs.ServiceName]structs.GatewayService{
		svc: {
			Service:  svc,
			CAFile:   "ca.pem",
			CertFile: "cert.pem",
			KeyFile:  "key.pem",
		},
	}

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Len(t, resources, 2)

	names := make(map[string]struct{}, 2)
	for _, r := range resources {
		names[r.(*envoy_tls_v3.Secret).Name] = struct{}{}
	}
	require.Contains(t, names, "my-special-service-cert")
	require.Contains(t, names, "my-special-service-ca")
}

func TestMakeUpstreamTLSContext_SecretNamesMatchServiceName(t *testing.T) {
	mapping := structs.GatewayService{
		Service: structs.NewServiceName("payments", structs.DefaultEnterpriseMetaInDefaultPartition()),
	}

	ctx := makeUpstreamTLSContext(mapping)

	require.NotNil(t, ctx)
	require.Len(t, ctx.TlsCertificateSdsSecretConfigs, 1)
	require.Equal(t, "payments-cert", ctx.TlsCertificateSdsSecretConfigs[0].Name)

	vc, ok := ctx.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_ValidationContextSdsSecretConfig)
	require.True(t, ok)
	require.Equal(t, "payments-ca", vc.ValidationContextSdsSecretConfig.Name)
}

func TestMakeUpstreamTLSContext_UsesSDS(t *testing.T) {
	mapping := structs.GatewayService{
		Service: structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition()),
	}

	ctx := makeUpstreamTLSContext(mapping)

	require.NotNil(t, ctx)

	certSDS := ctx.TlsCertificateSdsSecretConfigs[0].SdsConfig
	require.NotNil(t, certSDS)
	_, usesADS := certSDS.ConfigSourceSpecifier.(*envoy_core_v3.ConfigSource_Ads)
	require.True(t, usesADS, "cert SDS config should use ADS")
	require.Equal(t, envoy_core_v3.ApiVersion_V3, certSDS.ResourceApiVersion)

	vc := ctx.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_ValidationContextSdsSecretConfig)
	caSDS := vc.ValidationContextSdsSecretConfig.SdsConfig
	require.NotNil(t, caSDS)
	_, usesADS = caSDS.ConfigSourceSpecifier.(*envoy_core_v3.ConfigSource_Ads)
	require.True(t, usesADS, "CA SDS config should use ADS")
	require.Equal(t, envoy_core_v3.ApiVersion_V3, caSDS.ResourceApiVersion)
}

func TestMakeUpstreamTLSContext_DifferentServiceNames(t *testing.T) {
	tests := map[string]struct {
		serviceName  string
		wantCertName string
		wantCAName   string
	}{
		"simple name": {
			serviceName:  "db",
			wantCertName: "db-cert",
			wantCAName:   "db-ca",
		},
		"hyphenated name": {
			serviceName:  "my-service",
			wantCertName: "my-service-cert",
			wantCAName:   "my-service-ca",
		},
		"single char": {
			serviceName:  "a",
			wantCertName: "a-cert",
			wantCAName:   "a-ca",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			mapping := structs.GatewayService{
				Service: structs.NewServiceName(tt.serviceName, structs.DefaultEnterpriseMetaInDefaultPartition()),
			}

			ctx := makeUpstreamTLSContext(mapping)

			require.Equal(t, tt.wantCertName, ctx.TlsCertificateSdsSecretConfigs[0].Name)
			vc := ctx.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_ValidationContextSdsSecretConfig)
			require.Equal(t, tt.wantCAName, vc.ValidationContextSdsSecretConfig.Name)
		})
	}
}

func TestMakeUpstreamTLSContext_AlwaysHasBothCertAndValidationConfig(t *testing.T) {
	mapping := structs.GatewayService{
		Service: structs.NewServiceName("cache", structs.DefaultEnterpriseMetaInDefaultPartition()),
	}

	ctx := makeUpstreamTLSContext(mapping)

	require.NotNil(t, ctx)
	require.Len(t, ctx.TlsCertificateSdsSecretConfigs, 1, "should always have a cert SDS config")
	require.NotNil(t, ctx.ValidationContextType, "should always have a validation context")
}

func TestSecretsFromSnapshot_NonTerminatingGatewayKindsReturnNil(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.Kind = structs.ServiceKindConnectProxy

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Nil(t, resources)
}

func TestSecretsFromSnapshot_InvalidKindReturnsError(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.Kind = "not-a-real-kind"

	_, err := s.secretsFromSnapshot(snap)
	require.Error(t, err)
}
