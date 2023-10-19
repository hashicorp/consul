// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"math"

	"github.com/hashicorp/consul/internal/catalog"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/iptables"
)

func RegisterProxyConfiguration(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.ProxyConfigurationType,
		Proto:    &pbmesh.ProxyConfiguration{},
		Scope:    resource.ScopeNamespace,
		Mutate:   MutateProxyConfiguration,
		Validate: ValidateProxyConfiguration,
		ACLs:     catalog.ACLHooksForWorkloadSelectingType[*pbmesh.ProxyConfiguration](),
	})
}

func MutateProxyConfiguration(res *pbresource.Resource) error {
	var proxyCfg pbmesh.ProxyConfiguration
	err := res.Data.UnmarshalTo(&proxyCfg)
	if err != nil {
		return resource.NewErrDataParse(&proxyCfg, err)
	}

	changed := false

	// Default the tproxy outbound port.
	if proxyCfg.IsTransparentProxy() {
		if proxyCfg.GetDynamicConfig().GetTransparentProxy() == nil {
			proxyCfg.DynamicConfig.TransparentProxy = &pbmesh.TransparentProxy{
				OutboundListenerPort: iptables.DefaultTProxyOutboundPort,
			}
			changed = true
		} else if proxyCfg.GetDynamicConfig().GetTransparentProxy().OutboundListenerPort == 0 {
			proxyCfg.DynamicConfig.TransparentProxy.OutboundListenerPort = iptables.DefaultTProxyOutboundPort
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return res.Data.MarshalFrom(&proxyCfg)
}

func ValidateProxyConfiguration(res *pbresource.Resource) error {
	decodedProxyCfg, decodeErr := resource.Decode[*pbmesh.ProxyConfiguration](res)
	if decodeErr != nil {
		return resource.NewErrDataParse(decodedProxyCfg.GetData(), decodeErr)
	}
	proxyCfg := decodedProxyCfg.GetData()

	var err error

	if selErr := catalog.ValidateSelector(proxyCfg.Workloads, false); selErr != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "workloads",
			Wrapped: selErr,
		})
	}

	if proxyCfg.GetDynamicConfig() == nil && proxyCfg.GetBootstrapConfig() == nil {
		err = multierror.Append(err, resource.ErrInvalidFields{
			Names:   []string{"dynamic_config", "bootstrap_config"},
			Wrapped: errMissingProxyConfigData,
		})
	}

	// nolint:staticcheck
	if proxyCfg.GetOpaqueConfig() != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "opaque_config",
			Wrapped: resource.ErrUnsupported,
		})
	}

	if dynamicCfgErr := validateDynamicProxyConfiguration(proxyCfg.GetDynamicConfig()); dynamicCfgErr != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "dynamic_config",
			Wrapped: dynamicCfgErr,
		})
	}

	return err
}

func validateDynamicProxyConfiguration(cfg *pbmesh.DynamicConfig) error {
	if cfg == nil {
		return nil
	}

	var err error

	// Error if any of the currently unsupported fields is set.
	if cfg.GetMutualTlsMode() != pbmesh.MutualTLSMode_MUTUAL_TLS_MODE_DEFAULT {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "mutual_tls_mode",
			Wrapped: resource.ErrUnsupported,
		})
	}

	if cfg.GetMeshGatewayMode() != pbmesh.MeshGatewayMode_MESH_GATEWAY_MODE_UNSPECIFIED {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "mesh_gateway_mode",
			Wrapped: resource.ErrUnsupported,
		})
	}

	if cfg.GetAccessLogs() != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "access_logs",
			Wrapped: resource.ErrUnsupported,
		})
	}

	if cfg.GetPublicListenerJson() != "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "public_listener_json",
			Wrapped: resource.ErrUnsupported,
		})
	}

	if cfg.GetListenerTracingJson() != "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "listener_tracing_json",
			Wrapped: resource.ErrUnsupported,
		})
	}

	if cfg.GetLocalClusterJson() != "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "local_cluster_json",
			Wrapped: resource.ErrUnsupported,
		})
	}

	// nolint:staticcheck
	if cfg.GetLocalWorkloadAddress() != "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "local_workload_address",
			Wrapped: resource.ErrUnsupported,
		})
	}

	// nolint:staticcheck
	if cfg.GetLocalWorkloadPort() != 0 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "local_workload_port",
			Wrapped: resource.ErrUnsupported,
		})
	}

	// nolint:staticcheck
	if cfg.GetLocalWorkloadSocketPath() != "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "local_workload_socket_path",
			Wrapped: resource.ErrUnsupported,
		})
	}

	if tproxyCfg := cfg.GetTransparentProxy(); tproxyCfg != nil {
		if tproxyCfg.DialedDirectly {
			err = multierror.Append(err, resource.ErrInvalidField{
				Name: "transparent_proxy",
				Wrapped: resource.ErrInvalidField{
					Name:    "dialed_directly",
					Wrapped: resource.ErrUnsupported,
				},
			})
		}

		if outboundListenerPortErr := validatePort(tproxyCfg.OutboundListenerPort, "outbound_listener_port"); outboundListenerPortErr != nil {
			err = multierror.Append(err, resource.ErrInvalidField{
				Name:    "transparent_proxy",
				Wrapped: outboundListenerPortErr,
			})
		}
	}

	if exposeCfg := cfg.GetExposeConfig(); exposeCfg != nil {
		for i, path := range exposeCfg.GetExposePaths() {
			if listenerPortErr := validatePort(path.ListenerPort, "listener_port"); listenerPortErr != nil {
				err = multierror.Append(err, resource.ErrInvalidField{
					Name: "expose_config",
					Wrapped: resource.ErrInvalidListElement{
						Name:    "expose_paths",
						Index:   i,
						Wrapped: listenerPortErr,
					},
				})
			}

			if localPathPortErr := validatePort(path.LocalPathPort, "local_path_port"); localPathPortErr != nil {
				err = multierror.Append(err, resource.ErrInvalidField{
					Name: "expose_config",
					Wrapped: resource.ErrInvalidListElement{
						Name:    "expose_paths",
						Index:   i,
						Wrapped: localPathPortErr,
					},
				})
			}
		}
	}

	return err
}

func validatePort(port uint32, fieldName string) error {
	if port < 1 || port > math.MaxUint16 {
		return resource.ErrInvalidField{
			Name:    fieldName,
			Wrapped: errInvalidPort,
		}
	}
	return nil
}
