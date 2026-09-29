// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/config"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/private/prototest"
)

// fileBasedSecrets keeps only the secrets whose material comes from
// operator-supplied files. A terminating gateway now also emits Connect leaf
// and root secrets for its own downstream listener; the tests below target the
// upstream credential-injection secrets, so the Connect ones are filtered out.
func fileBasedSecrets(resources []proto.Message) []proto.Message {
	var out []proto.Message
	for _, res := range resources {
		secret, ok := res.(*envoy_tls_v3.Secret)
		if !ok {
			continue
		}
		switch t := secret.Type.(type) {
		case *envoy_tls_v3.Secret_TlsCertificate:
			if t.TlsCertificate.GetCertificateChain().GetFilename() != "" {
				out = append(out, res)
			}
		case *envoy_tls_v3.Secret_ValidationContext:
			if t.ValidationContext.GetTrustedCa().GetFilename() != "" {
				out = append(out, res)
			}
		}
	}
	return out
}

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
	resources = fileBasedSecrets(resources)
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
	resources = fileBasedSecrets(resources)
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
	resources = fileBasedSecrets(resources)
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
	resources = fileBasedSecrets(resources)
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
	resources = fileBasedSecrets(resources)
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
	resources = fileBasedSecrets(resources)
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
	resources = fileBasedSecrets(resources)
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
	resources = fileBasedSecrets(resources)
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
	resources = fileBasedSecrets(resources)
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
		Service:  structs.NewServiceName("payments", structs.DefaultEnterpriseMetaInDefaultPartition()),
		CAFile:   "ca.pem",
		CertFile: "cert.pem",
		KeyFile:  "key.pem",
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
		Service:  structs.NewServiceName("web", structs.DefaultEnterpriseMetaInDefaultPartition()),
		CAFile:   "ca.pem",
		CertFile: "cert.pem",
		KeyFile:  "key.pem",
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
				Service:  structs.NewServiceName(tt.serviceName, structs.DefaultEnterpriseMetaInDefaultPartition()),
				CAFile:   "ca.pem",
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
			}

			ctx := makeUpstreamTLSContext(mapping)

			require.Equal(t, tt.wantCertName, ctx.TlsCertificateSdsSecretConfigs[0].Name)
			vc := ctx.ValidationContextType.(*envoy_tls_v3.CommonTlsContext_ValidationContextSdsSecretConfig)
			require.Equal(t, tt.wantCAName, vc.ValidationContextSdsSecretConfig.Name)
		})
	}
}

func TestMakeUpstreamTLSContext_OneWayTLS_NoCertSDSConfig(t *testing.T) {
	mapping := structs.GatewayService{
		Service: structs.NewServiceName("cache", structs.DefaultEnterpriseMetaInDefaultPartition()),
		CAFile:  "ca.pem",
	}

	ctx := makeUpstreamTLSContext(mapping)

	require.NotNil(t, ctx)
	require.Empty(t, ctx.TlsCertificateSdsSecretConfigs, "one-way TLS must not request a client cert SDS secret")
	require.NotNil(t, ctx.ValidationContextType, "CA validation context must still be present")
}

func TestMakeUpstreamTLSContext_MTLS_HasBothCertAndValidationSDSConfig(t *testing.T) {
	mapping := structs.GatewayService{
		Service:  structs.NewServiceName("cache", structs.DefaultEnterpriseMetaInDefaultPartition()),
		CAFile:   "ca.pem",
		CertFile: "cert.pem",
		KeyFile:  "key.pem",
	}

	ctx := makeUpstreamTLSContext(mapping)

	require.NotNil(t, ctx)
	require.Len(t, ctx.TlsCertificateSdsSecretConfigs, 1, "mTLS must request a client cert SDS secret")
	require.NotNil(t, ctx.ValidationContextType, "CA validation context must be present for mTLS")
}

func TestMakeUpstreamTLSContext_NoFilesConfigured(t *testing.T) {
	mapping := structs.GatewayService{
		Service: structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition()),
	}

	ctx := makeUpstreamTLSContext(mapping)

	require.NotNil(t, ctx)
	require.Empty(t, ctx.TlsCertificateSdsSecretConfigs)
	require.NotNil(t, ctx.ValidationContextType)
}

func TestMakeUpstreamTLSContext_CertFileOnlyNoCertSDSConfig(t *testing.T) {
	mapping := structs.GatewayService{
		Service:  structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition()),
		CAFile:   "ca.pem",
		CertFile: "cert.pem",
	}

	ctx := makeUpstreamTLSContext(mapping)

	require.NotNil(t, ctx)
	require.Empty(t, ctx.TlsCertificateSdsSecretConfigs, "cert SDS must be absent when KeyFile is missing")
}

func TestMakeUpstreamTLSContext_KeyFileOnlyNoCertSDSConfig(t *testing.T) {
	mapping := structs.GatewayService{
		Service: structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition()),
		CAFile:  "ca.pem",
		KeyFile: "key.pem",
	}

	ctx := makeUpstreamTLSContext(mapping)

	require.NotNil(t, ctx)
	require.Empty(t, ctx.TlsCertificateSdsSecretConfigs, "cert SDS must be absent when CertFile is missing")
}

func TestMakeUpstreamTLSContext_CertAndKeyWithoutCAFile(t *testing.T) {
	mapping := structs.GatewayService{
		Service:  structs.NewServiceName("db", structs.DefaultEnterpriseMetaInDefaultPartition()),
		CertFile: "cert.pem",
		KeyFile:  "key.pem",
	}

	ctx := makeUpstreamTLSContext(mapping)

	require.NotNil(t, ctx)
	require.Len(t, ctx.TlsCertificateSdsSecretConfigs, 1, "cert SDS must be present when CertFile and KeyFile are both set")
	require.NotNil(t, ctx.ValidationContextType)
}

func TestSecretsFromSnapshot_NonTerminatingGatewayKindsReturnNil(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.Kind = structs.ServiceKindIngressGateway

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Nil(t, resources)
}

func TestSecretsFromSnapshot_ConnectProxyReturnsLeafAndRootSecrets(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	snap := proxycfg.TestConfigSnapshot(t, nil, nil)

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)
	require.Len(t, resources, 2)

	secrets := make(map[string]*envoy_tls_v3.Secret, len(resources))
	for _, resource := range resources {
		secret := resource.(*envoy_tls_v3.Secret)
		secrets[secret.Name] = secret
	}

	leafSecret := secrets[connectLeafSecretName]
	require.NotNil(t, leafSecret)
	leafCert, ok := leafSecret.Type.(*envoy_tls_v3.Secret_TlsCertificate)
	require.True(t, ok)
	require.Equal(t, lib.EnsureTrailingNewline(snap.Leaf().CertPEM), leafCert.TlsCertificate.CertificateChain.GetInlineString())
	require.Equal(t, lib.EnsureTrailingNewline(snap.Leaf().PrivateKeyPEM), leafCert.TlsCertificate.PrivateKey.GetInlineString())

	rootSecret := secrets[connectRootSecretName]
	require.NotNil(t, rootSecret)
	rootCA, ok := rootSecret.Type.(*envoy_tls_v3.Secret_ValidationContext)
	require.True(t, ok)
	require.Equal(t, snap.RootPEMs(), rootCA.ValidationContext.TrustedCa.GetInlineString())
}

// TestSecretsFromSnapshot_ConnectProxyEmitsEachSecretIndependently covers the
// degraded states where the snapshot is valid but only part of the mTLS
// material is present: a mesh gateway with no exported services (which cancels
// its leaf watch and sets Leaf to nil while keeping valid CA roots), and a root
// watch that has produced no CA roots.
//
// Each secret is emitted independently when its material is available.
// A nil leaf omits only connect-leaf, while empty roots omit only connect-root;
// neither case should panic or emit an invalid empty PEM.
func TestSecretsFromSnapshot_ConnectProxyEmitsEachSecretIndependently(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	secretNames := func(t *testing.T, resources []proto.Message) []string {
		t.Helper()
		var names []string
		for _, res := range resources {
			secret, ok := res.(*envoy_tls_v3.Secret)
			require.True(t, ok)
			names = append(names, secret.Name)
		}
		return names
	}

	// A mesh gateway with no exported services cancels its leaf watch and sets
	// Leaf to nil while keeping valid CA roots. The root secret must still be
	// delivered in that state, and reading the nil leaf must not panic.
	t.Run("nil leaf still emits the root secret", func(t *testing.T) {
		snap := proxycfg.TestConfigSnapshot(t, nil, nil)
		snap.ConnectProxy.Leaf = nil
		require.NotEmpty(t, snap.RootPEMs())

		require.NotPanics(t, func() {
			resources, err := s.secretsFromSnapshot(snap)
			require.NoError(t, err)
			require.Equal(t, []string{connectRootSecretName}, secretNames(t, resources))
		})
	})

	t.Run("no CA roots still emits the leaf secret", func(t *testing.T) {
		snap := proxycfg.TestConfigSnapshot(t, nil, nil)
		snap.Roots.Roots = nil
		require.Empty(t, snap.RootPEMs())
		require.NotNil(t, snap.Leaf())

		resources, err := s.secretsFromSnapshot(snap)
		require.NoError(t, err)
		require.Equal(t, []string{connectLeafSecretName}, secretNames(t, resources))
	})

	// An empty trusted CA is rejected by Envoy, so the root secret must be
	// withheld rather than emitted with an empty inline string.
	t.Run("neither available yields no secrets", func(t *testing.T) {
		snap := proxycfg.TestConfigSnapshot(t, nil, nil)
		snap.ConnectProxy.Leaf = nil
		snap.Roots.Roots = nil

		resources, err := s.secretsFromSnapshot(snap)
		require.NoError(t, err)
		require.Empty(t, resources)
	})
}

// TestCreateDownstreamTransportSocketForConnectTLS_StableWhileMaterialMissing
// asserts that the public listener's transport socket is byte-identical
// whether or not the leaf and roots are currently available.
//
// This is what makes a transient certificate gap non-disruptive: the listener
// proto does not change, so its filter chain hash does not change and Envoy
// does not drain established connections. Gating listener generation on
// certificate readiness would break this property and reintroduce the drain
// that this SDS change exists to eliminate.
func TestCreateDownstreamTransportSocketForConnectTLS_StableWhileMaterialMissing(t *testing.T) {
	build := func(mutate func(*proxycfg.ConfigSnapshot)) *envoy_core_v3.TransportSocket {
		snap := proxycfg.TestConfigSnapshot(t, nil, nil)
		if mutate != nil {
			mutate(snap)
		}

		ts, err := createDownstreamTransportSocketForConnectTLS(snap, &config.ProxyConfig{Protocol: "tcp"}, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, ts, "public listener must still be given a TLS transport socket")
		return ts
	}

	ready := build(nil)

	t.Run("identical when leaf is missing", func(t *testing.T) {
		got := build(func(snap *proxycfg.ConfigSnapshot) { snap.ConnectProxy.Leaf = nil })
		prototest.AssertDeepEqual(t, ready, got)
	})

	t.Run("identical when CA roots are missing", func(t *testing.T) {
		got := build(func(snap *proxycfg.ConfigSnapshot) { snap.Roots.Roots = nil })
		prototest.AssertDeepEqual(t, ready, got)
	})
}

func TestSecretsFromSnapshot_InvalidKindReturnsError(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}

	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
	snap.Kind = "not-a-real-kind"

	_, err := s.secretsFromSnapshot(snap)
	require.Error(t, err)
}

// TestSecretsFromSnapshotTerminatingGateway_EmitsDownstreamConnectSecrets
// covers the gateway's own public listener. Each linked service presents its
// own Connect leaf, so there is one leaf secret per service plus a single
// shared root secret, and every name the listener references must be backed by
// an emitted secret.
func TestSecretsFromSnapshotTerminatingGateway_EmitsDownstreamConnectSecrets(t *testing.T) {
	s := &ResourceGenerator{Logger: hclog.NewNullLogger()}
	snap := proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)

	resources, err := s.secretsFromSnapshot(snap)
	require.NoError(t, err)

	emitted := make(map[string]*envoy_tls_v3.Secret)
	for _, res := range resources {
		secret, ok := res.(*envoy_tls_v3.Secret)
		require.True(t, ok)
		emitted[secret.Name] = secret
	}

	// One leaf per linked service, carrying that service's own certificate.
	services := snap.TerminatingGateway.ValidServices()
	require.NotEmpty(t, services)
	for _, svc := range services {
		name := terminatingGatewayLeafSecretName(svc)
		secret, ok := emitted[name]
		require.True(t, ok, "expected a leaf secret named %q", name)

		cert, ok := secret.Type.(*envoy_tls_v3.Secret_TlsCertificate)
		require.True(t, ok)
		leaf := snap.TerminatingGateway.ServiceLeaves[svc]
		require.Equal(t, leaf.CertPEM, cert.TlsCertificate.CertificateChain.GetInlineString())
		require.Equal(t, leaf.PrivateKeyPEM, cert.TlsCertificate.PrivateKey.GetInlineString())
	}

	// The roots are common to every chain, so exactly one shared secret backs them.
	root, ok := emitted[connectRootSecretName]
	require.True(t, ok, "expected a shared root secret")
	vc, ok := root.Type.(*envoy_tls_v3.Secret_ValidationContext)
	require.True(t, ok)
	require.Equal(t, snap.RootPEMs(), vc.ValidationContext.TrustedCa.GetInlineString())

	// Guard against a listener referring to a secret that is never delivered.
	listeners, err := s.listenersFromSnapshot(snap)
	require.NoError(t, err)
	referenced := 0
	for _, l := range listeners {
		for _, name := range sdsSecretNamesInListener(t, l) {
			require.Contains(t, emitted, name, "listener references undelivered secret %q", name)
			referenced++
		}
	}
	require.NotZero(t, referenced, "expected the gateway listener to reference SDS secrets")
}

// sdsSecretNamesInListener returns every SDS secret name referenced by a
// listener's downstream transport sockets.
func sdsSecretNamesInListener(t *testing.T, res proto.Message) []string {
	t.Helper()

	l, ok := res.(*envoy_listener_v3.Listener)
	if !ok {
		return nil
	}

	var names []string
	for _, fc := range l.FilterChains {
		ts := fc.GetTransportSocket()
		if ts == nil {
			continue
		}
		var downstream envoy_tls_v3.DownstreamTlsContext
		require.NoError(t, ts.GetTypedConfig().UnmarshalTo(&downstream))

		ctx := downstream.GetCommonTlsContext()
		for _, sc := range ctx.GetTlsCertificateSdsSecretConfigs() {
			names = append(names, sc.Name)
		}
		if vc := ctx.GetValidationContextSdsSecretConfig(); vc != nil {
			names = append(names, vc.Name)
		}
	}
	return names
}
