package structs

import "github.com/mitchellh/go-testing-interface"

// TestConnectProxyConfig returns a ConnectProxyConfig representing a valid
// Connect proxy.
func TestConnectProxyConfig(t testing.T) ConnectProxyConfig {
	return ConnectProxyConfig{
		DestinationServiceName: "web",
		Upstreams:              TestUpstreams(t),
	}
}

// TestUpstreams returns a set of upstreams to be used in tests exercising most
// important configuration patterns.
func TestUpstreams(t testing.T) Upstreams {
	return Upstreams{
		{
			DestinationType: UpstreamDestTypeService,
			DestinationName: "db",
			LocalBindPort:   9191,
			Config: map[string]interface{}{
				// Float because this is how it is decoded by JSON decoder so this
				// enables the value returned to be compared directly to a decoded JSON
				// response without spurios type loss.
				"connect_timeout_ms": float64(1000),
			},
		},
		{
			DestinationType:  UpstreamDestTypePreparedQuery,
			DestinationName:  "geo-cache",
			LocalBindPort:    8181,
			LocalBindAddress: "127.10.10.10",
		},
	}
}
