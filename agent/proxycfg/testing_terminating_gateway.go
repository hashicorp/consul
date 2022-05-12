package proxycfg

import (
	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/cache"
	agentcache "github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

func TestConfigSnapshotTerminatingGateway(
	t testing.T,
	populateServices bool,
	nsFn func(ns *structs.NodeService),
	extraUpdates []agentcache.UpdateEvent,
) *ConfigSnapshot {
	roots, _ := TestCerts(t)

	var (
		web   = structs.NewServiceName("web", nil)
		api   = structs.NewServiceName("api", nil)
		db    = structs.NewServiceName("db", nil)
		cache = structs.NewServiceName("cache", nil)
	)

	baseEvents := []agentcache.UpdateEvent{
		{
			CorrelationID: rootsWatchID,
			Result:        roots,
		},
		{
			CorrelationID: gatewayServicesWatchID,
			Result: &structs.IndexedGatewayServices{
				Services: nil,
			},
		},
	}

	if populateServices {
		webNodes := TestUpstreamNodes(t, web.Name)
		webNodes[0].Service.Meta = map[string]string{"version": "1"}
		webNodes[1].Service.Meta = map[string]string{"version": "2"}

		apiNodes := structs.CheckServiceNodes{
			structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         "api",
					Node:       "test1",
					Address:    "10.10.1.1",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "api",
					Address: "api.mydomain",
					Port:    8081,
				},
				Checks: structs.HealthChecks{
					{Status: "critical"},
				},
			},
			structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         "test2",
					Node:       "test2",
					Address:    "10.10.1.2",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "api",
					Address: "api.altdomain",
					Port:    8081,
					Meta: map[string]string{
						"domain": "alt",
					},
				},
			},
			structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         "test3",
					Node:       "test3",
					Address:    "10.10.1.3",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "api",
					Address: "10.10.1.3",
					Port:    8081,
				},
			},
			structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         "test4",
					Node:       "test4",
					Address:    "10.10.1.4",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "api",
					Address: "api.thirddomain",
					Port:    8081,
				},
			},
		}

		// Has failing instance
		dbNodes := structs.CheckServiceNodes{
			structs.CheckServiceNode{
				Node: &structs.Node{
					ID:         "db",
					Node:       "test4",
					Address:    "10.10.1.4",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "db",
					Address: "db.mydomain",
					Port:    8081,
				},
				Checks: structs.HealthChecks{
					{Status: "critical"},
				},
			},
		}

		// Has passing instance but failing subset
		cacheNodes := structs.CheckServiceNodes{
			{
				Node: &structs.Node{
					ID:         "cache",
					Node:       "test5",
					Address:    "10.10.1.5",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "cache",
					Address: "cache.mydomain",
					Port:    8081,
				},
			},
			{
				Node: &structs.Node{
					ID:         "cache",
					Node:       "test5",
					Address:    "10.10.1.5",
					Datacenter: "dc1",
				},
				Service: &structs.NodeService{
					Service: "cache",
					Address: "cache.mydomain",
					Port:    8081,
					Meta: map[string]string{
						"Env": "prod",
					},
				},
				Checks: structs.HealthChecks{
					{Status: "critical"},
				},
			},
		}

		baseEvents = testSpliceEvents(baseEvents, []agentcache.UpdateEvent{
			{
				CorrelationID: gatewayServicesWatchID,
				Result: &structs.IndexedGatewayServices{
					Services: []*structs.GatewayService{
						{
							Service: web,
							CAFile:  "ca.cert.pem",
						},
						{
							Service:  api,
							CAFile:   "ca.cert.pem",
							CertFile: "api.cert.pem",
							KeyFile:  "api.key.pem",
						},
						{
							Service: db,
						},
						{
							Service: cache,
						},
					},
				},
			},
			{
				CorrelationID: externalServiceIDPrefix + web.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: webNodes,
				},
			},
			{
				CorrelationID: externalServiceIDPrefix + api.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: apiNodes,
				},
			},
			{
				CorrelationID: externalServiceIDPrefix + db.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: dbNodes,
				},
			},
			{
				CorrelationID: externalServiceIDPrefix + cache.String(),
				Result: &structs.IndexedCheckServiceNodes{
					Nodes: cacheNodes,
				},
			},
			// ========
			// no intentions defined for these services
			{
				CorrelationID: serviceIntentionsIDPrefix + web.String(),
				Result: &structs.IndexedIntentionMatches{
					Matches: []structs.Intentions{
						nil,
					},
				},
			},
			{
				CorrelationID: serviceIntentionsIDPrefix + api.String(),
				Result: &structs.IndexedIntentionMatches{
					Matches: []structs.Intentions{
						nil,
					},
				},
			},
			{
				CorrelationID: serviceIntentionsIDPrefix + db.String(),
				Result: &structs.IndexedIntentionMatches{
					Matches: []structs.Intentions{
						nil,
					},
				},
			},
			{
				CorrelationID: serviceIntentionsIDPrefix + cache.String(),
				Result: &structs.IndexedIntentionMatches{
					Matches: []structs.Intentions{
						nil,
					},
				},
			},
			// ========
			{
				CorrelationID: serviceLeafIDPrefix + web.String(),
				Result: &structs.IssuedCert{
					CertPEM:       golden(t, "test-leaf-cert"),
					PrivateKeyPEM: golden(t, "test-leaf-key"),
				},
			},
			{
				CorrelationID: serviceLeafIDPrefix + api.String(),
				Result: &structs.IssuedCert{
					CertPEM:       golden(t, "alt-test-leaf-cert"),
					PrivateKeyPEM: golden(t, "alt-test-leaf-key"),
				},
			},
			{
				CorrelationID: serviceLeafIDPrefix + db.String(),
				Result: &structs.IssuedCert{
					CertPEM:       golden(t, "db-test-leaf-cert"),
					PrivateKeyPEM: golden(t, "db-test-leaf-key"),
				},
			},
			{
				CorrelationID: serviceLeafIDPrefix + cache.String(),
				Result: &structs.IssuedCert{
					CertPEM:       golden(t, "cache-test-leaf-cert"),
					PrivateKeyPEM: golden(t, "cache-test-leaf-key"),
				},
			},
			// ========
			{
				CorrelationID: serviceConfigIDPrefix + web.String(),
				Result: &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "tcp"},
				},
			},
			{
				CorrelationID: serviceConfigIDPrefix + api.String(),
				Result: &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "tcp"},
				},
			},
			{
				CorrelationID: serviceConfigIDPrefix + db.String(),
				Result: &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "tcp"},
				},
			},
			{
				CorrelationID: serviceConfigIDPrefix + cache.String(),
				Result: &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "tcp"},
				},
			},
			// ========
			{
				CorrelationID: serviceResolverIDPrefix + web.String(),
				Result: &structs.ConfigEntryResponse{
					Entry: &structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
					},
				},
			},
			{
				CorrelationID: serviceResolverIDPrefix + api.String(),
				Result: &structs.ConfigEntryResponse{
					Entry: &structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
					},
				},
			},
			{
				CorrelationID: serviceResolverIDPrefix + db.String(),
				Result: &structs.ConfigEntryResponse{
					Entry: &structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
					},
				},
			},
			{
				CorrelationID: serviceResolverIDPrefix + cache.String(),
				Result: &structs.ConfigEntryResponse{
					Entry: &structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
					},
				},
			},
		})
	}

	return testConfigSnapshotFixture(t, &structs.NodeService{
		Kind:    structs.ServiceKindTerminatingGateway,
		Service: "terminating-gateway",
		Address: "1.2.3.4",
		Port:    8443,
		TaggedAddresses: map[string]structs.ServiceAddress{
			structs.TaggedAddressWAN: {
				Address: "198.18.0.1",
				Port:    443,
			},
		},
	}, nsFn, nil, testSpliceEvents(baseEvents, extraUpdates))
}

func TestConfigSnapshotTerminatingGatewayServiceSubsets(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotTerminatingGatewayServiceSubsets(t, false)
}
func TestConfigSnapshotTerminatingGatewayServiceSubsetsWebAndCache(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotTerminatingGatewayServiceSubsets(t, true)
}
func testConfigSnapshotTerminatingGatewayServiceSubsets(t testing.T, alsoAdjustCache bool) *ConfigSnapshot {
	var (
		web   = structs.NewServiceName("web", nil)
		cache = structs.NewServiceName("cache", nil)
	)

	events := []agentcache.UpdateEvent{
		{
			CorrelationID: serviceResolverIDPrefix + web.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "web",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == 1",
						},
						"v2": {
							Filter:      "Service.Meta.version == 2",
							OnlyPassing: true,
						},
					},
				},
			},
		},
		{
			CorrelationID: serviceConfigIDPrefix + web.String(),
			Result: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{"protocol": "http"},
			},
		},
	}

	if alsoAdjustCache {
		events = testSpliceEvents(events, []agentcache.UpdateEvent{
			{
				CorrelationID: serviceResolverIDPrefix + cache.String(),
				Result: &structs.ConfigEntryResponse{
					Entry: &structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "cache",
						Subsets: map[string]structs.ServiceResolverSubset{
							"prod": {
								Filter: "Service.Meta.Env == prod",
							},
						},
					},
				},
			},
			{
				CorrelationID: serviceConfigIDPrefix + web.String(),
				Result: &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				},
			},
		})
	}

	return TestConfigSnapshotTerminatingGateway(t, true, nil, events)
}

func TestConfigSnapshotTerminatingGatewayDefaultServiceSubset(t testing.T) *ConfigSnapshot {
	web := structs.NewServiceName("web", nil)

	return TestConfigSnapshotTerminatingGateway(t, true, nil, []agentcache.UpdateEvent{
		{
			CorrelationID: serviceResolverIDPrefix + web.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.ServiceResolverConfigEntry{
					Kind:          structs.ServiceResolver,
					Name:          "web",
					DefaultSubset: "v2",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.version == 1",
						},
						"v2": {
							Filter:      "Service.Meta.version == 2",
							OnlyPassing: true,
						},
					},
				},
			},
		},
		// {
		// 	CorrelationID: serviceConfigIDPrefix + web.String(),
		// 	Result: &structs.ServiceConfigResponse{
		// 		ProxyConfig: map[string]interface{}{"protocol": "http"},
		// 	},
		// },
	})
}

func TestConfigSnapshotTerminatingGatewayLBConfig(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotTerminatingGatewayLBConfig(t, "default")
}
func TestConfigSnapshotTerminatingGatewayLBConfigNoHashPolicies(t testing.T) *ConfigSnapshot {
	return testConfigSnapshotTerminatingGatewayLBConfig(t, "no-hash-policies")
}
func testConfigSnapshotTerminatingGatewayLBConfig(t testing.T, variant string) *ConfigSnapshot {
	web := structs.NewServiceName("web", nil)

	entry := &structs.ServiceResolverConfigEntry{
		Kind:          structs.ServiceResolver,
		Name:          "web",
		DefaultSubset: "v2",
		Subsets: map[string]structs.ServiceResolverSubset{
			"v1": {
				Filter: "Service.Meta.Version == 1",
			},
			"v2": {
				Filter:      "Service.Meta.Version == 2",
				OnlyPassing: true,
			},
		},
		LoadBalancer: &structs.LoadBalancer{
			Policy: "ring_hash",
			RingHashConfig: &structs.RingHashConfig{
				MinimumRingSize: 20,
				MaximumRingSize: 50,
			},
			HashPolicies: []structs.HashPolicy{
				{
					Field:      structs.HashPolicyCookie,
					FieldValue: "chocolate-chip",
					Terminal:   true,
				},
				{
					Field:      structs.HashPolicyHeader,
					FieldValue: "x-user-id",
				},
				{
					SourceIP: true,
					Terminal: true,
				},
			},
		},
	}

	switch variant {
	case "default":
	case "no-hash-policies":
		entry.LoadBalancer.HashPolicies = nil
	default:
		t.Fatalf("unknown variant %q", variant)
		return nil
	}

	return TestConfigSnapshotTerminatingGateway(t, true, nil, []cache.UpdateEvent{
		{
			CorrelationID: serviceConfigIDPrefix + web.String(),
			Result: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{"protocol": "http"},
			},
		},
		{
			CorrelationID: serviceResolverIDPrefix + web.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: entry,
			},
		},
		{
			CorrelationID: serviceConfigIDPrefix + web.String(),
			Result: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{"protocol": "http"},
			},
		},
	})
}

func TestConfigSnapshotTerminatingGatewaySNI(t testing.T) *ConfigSnapshot {
	return TestConfigSnapshotTerminatingGateway(t, true, nil, []cache.UpdateEvent{
		{
			CorrelationID: "gateway-services",
			Result: &structs.IndexedGatewayServices{
				Services: []*structs.GatewayService{
					{
						Service: structs.NewServiceName("web", nil),
						CAFile:  "ca.cert.pem",
						SNI:     "foo.com",
					},
					{
						Service:  structs.NewServiceName("api", nil),
						CAFile:   "ca.cert.pem",
						CertFile: "api.cert.pem",
						KeyFile:  "api.key.pem",
						SNI:      "bar.com",
					},
				},
			},
		},
	})
}

func TestConfigSnapshotTerminatingGatewayHostnameSubsets(t testing.T) *ConfigSnapshot {
	var (
		api   = structs.NewServiceName("api", nil)
		cache = structs.NewServiceName("cache", nil)
	)

	return TestConfigSnapshotTerminatingGateway(t, true, nil, []agentcache.UpdateEvent{
		{
			CorrelationID: serviceResolverIDPrefix + api.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "api",
					Subsets: map[string]structs.ServiceResolverSubset{
						"alt": {
							Filter: "Service.Meta.domain == alt",
						},
					},
				},
			},
		},
		{
			CorrelationID: serviceResolverIDPrefix + cache.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: "cache",
					Subsets: map[string]structs.ServiceResolverSubset{
						"prod": {
							Filter: "Service.Meta.Env == prod",
						},
					},
				},
			},
		},
		{
			CorrelationID: serviceConfigIDPrefix + api.String(),
			Result: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{"protocol": "http"},
			},
		},
		{
			CorrelationID: serviceConfigIDPrefix + cache.String(),
			Result: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{"protocol": "http"},
			},
		},
	})
}

func TestConfigSnapshotTerminatingGatewayIgnoreExtraResolvers(t testing.T) *ConfigSnapshot {
	var (
		web      = structs.NewServiceName("web", nil)
		notfound = structs.NewServiceName("notfound", nil)
	)

	return TestConfigSnapshotTerminatingGateway(t, true, nil, []agentcache.UpdateEvent{
		{
			CorrelationID: serviceResolverIDPrefix + web.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.ServiceResolverConfigEntry{
					Kind:          structs.ServiceResolver,
					Name:          "web",
					DefaultSubset: "v2",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.Version == 1",
						},
						"v2": {
							Filter:      "Service.Meta.Version == 2",
							OnlyPassing: true,
						},
					},
				},
			},
		},
		{
			CorrelationID: serviceResolverIDPrefix + notfound.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.ServiceResolverConfigEntry{
					Kind:          structs.ServiceResolver,
					Name:          "notfound",
					DefaultSubset: "v2",
					Subsets: map[string]structs.ServiceResolverSubset{
						"v1": {
							Filter: "Service.Meta.Version == 1",
						},
						"v2": {
							Filter:      "Service.Meta.Version == 2",
							OnlyPassing: true,
						},
					},
				},
			},
		},
		{
			CorrelationID: serviceConfigIDPrefix + web.String(),
			Result: &structs.ServiceConfigResponse{
				ProxyConfig: map[string]interface{}{"protocol": "http"},
			},
		},
	})
}

func TestConfigSnapshotTerminatingGatewayWithLambdaService(t testing.T, extraUpdateEvents ...agentcache.UpdateEvent) *ConfigSnapshot {
	web := structs.NewServiceName("web", nil)
	updateEvents := append(extraUpdateEvents, agentcache.UpdateEvent{
		CorrelationID: serviceConfigIDPrefix + web.String(),
		Result: &structs.ServiceConfigResponse{
			ProxyConfig: map[string]interface{}{"protocol": "http"},
			Meta: map[string]string{
				"serverless.consul.hashicorp.com/v1alpha1/lambda/enabled":             "true",
				"serverless.consul.hashicorp.com/v1alpha1/lambda/arn":                 "lambda-arn",
				"serverless.consul.hashicorp.com/v1alpha1/lambda/payload-passthrough": "true",
				"serverless.consul.hashicorp.com/v1alpha1/lambda/region":              "us-east-1",
			},
		},
	})
	return TestConfigSnapshotTerminatingGateway(t, true, nil, updateEvents)
}

func TestConfigSnapshotTerminatingGatewayWithLambdaServiceAndServiceResolvers(t testing.T) *ConfigSnapshot {
	web := structs.NewServiceName("web", nil)

	return TestConfigSnapshotTerminatingGatewayWithLambdaService(t,
		agentcache.UpdateEvent{
			CorrelationID: serviceResolverIDPrefix + web.String(),
			Result: &structs.ConfigEntryResponse{
				Entry: &structs.ServiceResolverConfigEntry{
					Kind: structs.ServiceResolver,
					Name: web.String(),
					Subsets: map[string]structs.ServiceResolverSubset{
						"canary1": {},
						"canary2": {},
					},
				},
			},
		})
}
