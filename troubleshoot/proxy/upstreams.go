package troubleshoot

import (
	"fmt"

	envoy_admin_v3 "github.com/envoyproxy/go-control-plane/envoy/admin/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_resource_v3 "github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
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
		case listenersType:
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

					upstream_ips, err = getUpstreamIPsFromFilterChain(l.GetFilterChains(), t.envoyConfigDump)
					if err != nil {
						return nil, nil, err
					}
				}
			}
		}
	}
	return upstream_envoy_ids, upstream_ips, nil
}

func getUpstreamIPsFromFilterChain(filterChains []*envoy_listener_v3.FilterChain,
	cfgDump *envoy_admin_v3.ConfigDump) ([]UpstreamIP, error) {
	var err error
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

				if cfg != nil {
					clusterNames = extensioncommon.RouteClusterNames(cfg)
				} else {
					// If there are no route configs, look for RDS.
					routeName := config.GetRds().GetRouteConfigName()
					if routeName != "" {
						clusterNames, err = getClustersFromRoutes(routeName, cfgDump)
						if err != nil {
							return nil, fmt.Errorf("error in getting clusters for route %q: %w", routeName, err)
						}
					}
				}
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

func getClustersFromRoutes(routeName string, cfgDump *envoy_admin_v3.ConfigDump) (map[string]struct{}, error) {

	for _, cfg := range cfgDump.Configs {
		switch cfg.TypeUrl {
		case routesType:
			rcd := &envoy_admin_v3.RoutesConfigDump{}

			err := proto.Unmarshal(cfg.GetValue(), rcd)
			if err != nil {
				return nil, err
			}

			for _, route := range rcd.GetDynamicRouteConfigs() {

				routeConfig := &envoy_route_v3.RouteConfiguration{}
				err = proto.Unmarshal(route.GetRouteConfig().GetValue(), routeConfig)
				if err != nil {
					return nil, err
				}

				if routeConfig.GetName() == routeName {
					return extensioncommon.RouteClusterNames(routeConfig), nil
				}
			}
		}
	}
	return nil, nil
}
