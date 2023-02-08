package troubleshoot

import (
	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"

	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
	"google.golang.org/protobuf/proto"
)

type UpstreamIP struct {
	IPs          []string
	IsVirtual    bool
	ClusterNames map[string]struct{}
}

func (t *Troubleshoot) GetUpstreams() ([]string, []UpstreamIP, error) {

	upstream_envoy_ids := []string{}
	upstream_ips := []UpstreamIP{}

	err := t.GetEnvoyConfigDump()
	if err != nil {
		return nil, nil, err
	}

	for _, cfg := range t.envoyConfigDump.Configs {
		switch cfg.TypeUrl {
		case listeners:
			lcd := &envoy_admin_v3.ListenersConfigDump{}

			err := proto.Unmarshal(cfg.GetValue(), lcd)
			if err != nil {
				return nil, nil, err
			}

			for _, listener := range lcd.GetDynamicListeners() {

				eid := envoyID(listener.Name)

				if eid != "" && eid != "public_listener" &&
					eid != "outbound_listener" && eid != "inbound_listener" {
					upstream_envoy_ids = append(upstream_envoy_ids, eid)
				} else if eid == "outbound_listener" {
					l := &envoy_listener_v3.Listener{}
					err = proto.Unmarshal(listener.GetActiveState().GetListener().GetValue(), l)
					if err != nil {
						return nil, nil, err
					}

					upstream_ips, err = getUpstreamIPsFromFilterChain(l.GetFilterChains())
					if err != nil {
						return nil, nil, err
					}
				}
			}
		}
	}
	return upstream_envoy_ids, upstream_ips, nil
}

func getUpstreamIPsFromFilterChain(filterChains []*envoy_listener_v3.FilterChain) ([]UpstreamIP, error) {
	if filterChains == nil {
		return []UpstreamIP{}, nil
	}

	upstreamIPs := []UpstreamIP{}
	for _, fc := range filterChains {

		if fc.GetFilters() == nil {
			continue
		}

		if fc.GetFilterChainMatch() == nil {
			continue
		}
		if fc.GetFilterChainMatch().GetPrefixRanges() == nil {
			continue
		}

		cidrs := fc.GetFilterChainMatch().GetPrefixRanges()
		ips := []string{}

		for _, cidr := range cidrs {
			ips = append(ips, cidr.AddressPrefix)
		}

		for _, filter := range fc.GetFilters() {
			isVirtual := false

			if filter.GetTypedConfig() == nil {
				continue
			}

			clusterNames := map[string]struct{}{}

			if config := envoy_resource_v3.GetHTTPConnectionManager(filter); config != nil {
				isVirtual = true

				cfg := config.GetRouteConfig()

				clusterNames = extensioncommon.RouteClusterNames(cfg)
			}
			if config := extensioncommon.GetTCPProxy(filter); config != nil {
				if config.GetCluster() != "" {
					clusterNames[config.GetCluster()] = struct{}{}
				}
			}

			upstreamIPs = append(upstreamIPs, UpstreamIP{
				IPs:          ips,
				IsVirtual:    isVirtual,
				ClusterNames: clusterNames,
			})
		}
	}

	return upstreamIPs, nil
}
