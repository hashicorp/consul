package serverlessplugin

import (
	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

const (
	CONNECTION_BALANCE_KEY = "consul.hashicorp.com/v1alpha1/connect_proxy/listener/connection_balance_config"
)

var _ patcher = (*connectionBalancePatcher)(nil)

type connectionBalancePatcher struct {
	serviceConfig *xdscommon.ServiceConfig
}

// CanPatch implements patcher
func (*connectionBalancePatcher) CanPatch(kind api.ServiceKind) bool {
	return kind == api.ServiceKindConnectProxy
}

// PatchCluster implements patcher
func (*connectionBalancePatcher) PatchCluster(c *envoy_cluster_v3.Cluster) (*envoy_cluster_v3.Cluster, bool, error) {
	return c, false, nil
}

// PatchFilter implements patcher
func (*connectionBalancePatcher) PatchFilter(f *envoy_listener_v3.Filter) (*envoy_listener_v3.Filter, bool, error) {
	return f, false, nil
}

// PatchRoute implements patcher
func (*connectionBalancePatcher) PatchRoute(r *envoy_route_v3.RouteConfiguration) (*envoy_route_v3.RouteConfiguration, bool, error) {
	return r, false, nil
}

// PatchListener implements patcher
func (c *connectionBalancePatcher) PatchListener(l *envoy_listener_v3.Listener) (*envoy_listener_v3.Listener, bool, error) {
	if strategy := c.serviceConfig.Meta[CONNECTION_BALANCE_KEY]; strategy == "exact_balance" {
		l.ConnectionBalanceConfig = &envoy_listener_v3.Listener_ConnectionBalanceConfig{
			BalanceType: &envoy_listener_v3.Listener_ConnectionBalanceConfig_ExactBalance_{},
		}
		return l, true, nil
	}
	return l, false, nil
}

func makeLoadBalancePatcher(serviceConfig xdscommon.ServiceConfig) (patcher, bool) {
	return &connectionBalancePatcher{
		serviceConfig: &serviceConfig,
	}, true
}
