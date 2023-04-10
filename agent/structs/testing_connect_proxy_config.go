// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/acl"
)

// TestConnectProxyConfig returns a ConnectProxyConfig representing a valid
// Connect proxy.
func TestConnectProxyConfig(t testing.T) ConnectProxyConfig {
	return ConnectProxyConfig{
		DestinationServiceName: "web",
		Upstreams:              TestUpstreams(t, false),
	}
}

// TestUpstreams returns a set of upstreams to be used in tests exercising most
// important configuration patterns.
func TestUpstreams(t testing.T, enterprise bool) Upstreams {
	db := Upstream{
		// We rely on this one having default type in a few tests...
		DestinationName: "db",
		LocalBindPort:   9191,
		Config: map[string]interface{}{
			// Float because this is how it is decoded by JSON decoder so this
			// enables the value returned to be compared directly to a decoded JSON
			// response without spurious type loss.
			"connect_timeout_ms": float64(1000),
		},
	}

	geoCache := Upstream{
		DestinationType:  UpstreamDestTypePreparedQuery,
		DestinationName:  "geo-cache",
		LocalBindPort:    8181,
		LocalBindAddress: "127.10.10.10",
	}

	if enterprise {
		db.DestinationNamespace = "foo"
		db.DestinationPartition = "bar"

		geoCache.DestinationNamespace = "baz"
		geoCache.DestinationPartition = "qux"
	}

	return Upstreams{db, geoCache, {
		DestinationName:     "upstream_socket",
		LocalBindSocketPath: "/tmp/upstream.sock",
		LocalBindSocketMode: "0700",
	},
	}
}

// TestAddDefaultsToUpstreams takes an array of upstreams (such as that from
// TestUpstreams) and adds default values that are populated during
// registration. Use this for generating the expected Upstreams value after
// registration.
func TestAddDefaultsToUpstreams(t testing.T, upstreams []Upstream, entMeta acl.EnterpriseMeta) Upstreams {
	ups := make([]Upstream, len(upstreams))
	for i := range upstreams {
		ups[i] = upstreams[i]
		// Fill in default fields we expect to have back explicitly in a response
		if ups[i].DestinationType == "" {
			ups[i].DestinationType = UpstreamDestTypeService
		}
		if ups[i].DestinationNamespace == "" {
			ups[i].DestinationNamespace = entMeta.NamespaceOrEmpty()
		}
		if ups[i].DestinationPartition == "" {
			ups[i].DestinationPartition = entMeta.PartitionOrEmpty()
		}
	}
	return ups
}
