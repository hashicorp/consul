// generated by deep-copy -pointer-receiver -o ./structs.deepcopy.go -type APIGatewayListener -type BoundAPIGatewayListener -type CARoot -type CheckServiceNode -type CheckType -type CompiledDiscoveryChain -type ConnectProxyConfig -type DiscoveryFailover -type DiscoveryGraphNode -type DiscoveryResolver -type DiscoveryRoute -type DiscoverySplit -type ExposeConfig -type ExportedServicesConfigEntry -type GatewayService -type GatewayServiceTLSConfig -type HTTPHeaderModifiers -type HTTPRouteConfigEntry -type HashPolicy -type HealthCheck -type IndexedCARoots -type IngressListener -type InlineCertificateConfigEntry -type Intention -type IntentionPermission -type LoadBalancer -type MeshConfigEntry -type MeshDirectionalTLSConfig -type MeshTLSConfig -type Node -type NodeService -type PeeringServiceMeta -type ServiceConfigEntry -type ServiceConfigResponse -type ServiceConnect -type ServiceDefinition -type ServiceResolverConfigEntry -type ServiceResolverFailover -type ServiceRoute -type ServiceRouteDestination -type ServiceRouteMatch -type TCPRouteConfigEntry -type Upstream -type UpstreamConfiguration -type Status -type BoundAPIGatewayConfigEntry ./; DO NOT EDIT.

package structs

import (
	"github.com/hashicorp/consul/types"
	"time"
)

// DeepCopy generates a deep copy of *APIGatewayListener
func (o *APIGatewayListener) DeepCopy() *APIGatewayListener {
	var cp APIGatewayListener = *o
	if o.TLS.Certificates != nil {
		cp.TLS.Certificates = make([]ResourceReference, len(o.TLS.Certificates))
		copy(cp.TLS.Certificates, o.TLS.Certificates)
	}
	if o.TLS.CipherSuites != nil {
		cp.TLS.CipherSuites = make([]types.TLSCipherSuite, len(o.TLS.CipherSuites))
		copy(cp.TLS.CipherSuites, o.TLS.CipherSuites)
	}
	return &cp
}

// DeepCopy generates a deep copy of *BoundAPIGatewayListener
func (o *BoundAPIGatewayListener) DeepCopy() *BoundAPIGatewayListener {
	var cp BoundAPIGatewayListener = *o
	if o.Routes != nil {
		cp.Routes = make([]ResourceReference, len(o.Routes))
		copy(cp.Routes, o.Routes)
	}
	if o.Certificates != nil {
		cp.Certificates = make([]ResourceReference, len(o.Certificates))
		copy(cp.Certificates, o.Certificates)
	}
	return &cp
}

// DeepCopy generates a deep copy of *CARoot
func (o *CARoot) DeepCopy() *CARoot {
	var cp CARoot = *o
	if o.IntermediateCerts != nil {
		cp.IntermediateCerts = make([]string, len(o.IntermediateCerts))
		copy(cp.IntermediateCerts, o.IntermediateCerts)
	}
	return &cp
}

// DeepCopy generates a deep copy of *CheckServiceNode
func (o *CheckServiceNode) DeepCopy() *CheckServiceNode {
	var cp CheckServiceNode = *o
	if o.Node != nil {
		cp.Node = o.Node.DeepCopy()
	}
	if o.Service != nil {
		cp.Service = o.Service.DeepCopy()
	}
	if o.Checks != nil {
		cp.Checks = make([]*HealthCheck, len(o.Checks))
		copy(cp.Checks, o.Checks)
		for i2 := range o.Checks {
			if o.Checks[i2] != nil {
				cp.Checks[i2] = o.Checks[i2].DeepCopy()
			}
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *CheckType
func (o *CheckType) DeepCopy() *CheckType {
	var cp CheckType = *o
	if o.ScriptArgs != nil {
		cp.ScriptArgs = make([]string, len(o.ScriptArgs))
		copy(cp.ScriptArgs, o.ScriptArgs)
	}
	if o.Header != nil {
		cp.Header = make(map[string][]string, len(o.Header))
		for k2, v2 := range o.Header {
			var cp_Header_v2 []string
			if v2 != nil {
				cp_Header_v2 = make([]string, len(v2))
				copy(cp_Header_v2, v2)
			}
			cp.Header[k2] = cp_Header_v2
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *CompiledDiscoveryChain
func (o *CompiledDiscoveryChain) DeepCopy() *CompiledDiscoveryChain {
	var cp CompiledDiscoveryChain = *o
	if o.ServiceMeta != nil {
		cp.ServiceMeta = make(map[string]string, len(o.ServiceMeta))
		for k2, v2 := range o.ServiceMeta {
			cp.ServiceMeta[k2] = v2
		}
	}
	if o.EnvoyExtensions != nil {
		cp.EnvoyExtensions = make([]EnvoyExtension, len(o.EnvoyExtensions))
		copy(cp.EnvoyExtensions, o.EnvoyExtensions)
		for i2 := range o.EnvoyExtensions {
			if o.EnvoyExtensions[i2].Arguments != nil {
				cp.EnvoyExtensions[i2].Arguments = make(map[string]interface{}, len(o.EnvoyExtensions[i2].Arguments))
				for k4, v4 := range o.EnvoyExtensions[i2].Arguments {
					cp.EnvoyExtensions[i2].Arguments[k4] = v4
				}
			}
		}
	}
	if o.Nodes != nil {
		cp.Nodes = make(map[string]*DiscoveryGraphNode, len(o.Nodes))
		for k2, v2 := range o.Nodes {
			var cp_Nodes_v2 *DiscoveryGraphNode
			if v2 != nil {
				cp_Nodes_v2 = v2.DeepCopy()
			}
			cp.Nodes[k2] = cp_Nodes_v2
		}
	}
	if o.Targets != nil {
		cp.Targets = make(map[string]*DiscoveryTarget, len(o.Targets))
		for k2, v2 := range o.Targets {
			var cp_Targets_v2 *DiscoveryTarget
			if v2 != nil {
				cp_Targets_v2 = new(DiscoveryTarget)
				*cp_Targets_v2 = *v2
				if v2.Locality != nil {
					cp_Targets_v2.Locality = new(Locality)
					*cp_Targets_v2.Locality = *v2.Locality
				}
			}
			cp.Targets[k2] = cp_Targets_v2
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *ConnectProxyConfig
func (o *ConnectProxyConfig) DeepCopy() *ConnectProxyConfig {
	var cp ConnectProxyConfig = *o
	if o.EnvoyExtensions != nil {
		cp.EnvoyExtensions = make([]EnvoyExtension, len(o.EnvoyExtensions))
		copy(cp.EnvoyExtensions, o.EnvoyExtensions)
		for i2 := range o.EnvoyExtensions {
			if o.EnvoyExtensions[i2].Arguments != nil {
				cp.EnvoyExtensions[i2].Arguments = make(map[string]interface{}, len(o.EnvoyExtensions[i2].Arguments))
				for k4, v4 := range o.EnvoyExtensions[i2].Arguments {
					cp.EnvoyExtensions[i2].Arguments[k4] = v4
				}
			}
		}
	}
	if o.Config != nil {
		cp.Config = make(map[string]interface{}, len(o.Config))
		for k2, v2 := range o.Config {
			cp.Config[k2] = v2
		}
	}
	if o.Upstreams != nil {
		cp.Upstreams = make([]Upstream, len(o.Upstreams))
		copy(cp.Upstreams, o.Upstreams)
		for i2 := range o.Upstreams {
			{
				retV := o.Upstreams[i2].DeepCopy()
				cp.Upstreams[i2] = *retV
			}
		}
	}
	{
		retV := o.Expose.DeepCopy()
		cp.Expose = *retV
	}
	return &cp
}

// DeepCopy generates a deep copy of *DiscoveryFailover
func (o *DiscoveryFailover) DeepCopy() *DiscoveryFailover {
	var cp DiscoveryFailover = *o
	if o.Targets != nil {
		cp.Targets = make([]string, len(o.Targets))
		copy(cp.Targets, o.Targets)
	}
	if o.Policy != nil {
		cp.Policy = new(ServiceResolverFailoverPolicy)
		*cp.Policy = *o.Policy
		if o.Policy.Regions != nil {
			cp.Policy.Regions = make([]string, len(o.Policy.Regions))
			copy(cp.Policy.Regions, o.Policy.Regions)
		}
	}
	if o.Regions != nil {
		cp.Regions = make([]string, len(o.Regions))
		copy(cp.Regions, o.Regions)
	}
	return &cp
}

// DeepCopy generates a deep copy of *DiscoveryGraphNode
func (o *DiscoveryGraphNode) DeepCopy() *DiscoveryGraphNode {
	var cp DiscoveryGraphNode = *o
	if o.Routes != nil {
		cp.Routes = make([]*DiscoveryRoute, len(o.Routes))
		copy(cp.Routes, o.Routes)
		for i2 := range o.Routes {
			if o.Routes[i2] != nil {
				cp.Routes[i2] = o.Routes[i2].DeepCopy()
			}
		}
	}
	if o.Splits != nil {
		cp.Splits = make([]*DiscoverySplit, len(o.Splits))
		copy(cp.Splits, o.Splits)
		for i2 := range o.Splits {
			if o.Splits[i2] != nil {
				cp.Splits[i2] = o.Splits[i2].DeepCopy()
			}
		}
	}
	if o.Resolver != nil {
		cp.Resolver = o.Resolver.DeepCopy()
	}
	if o.LoadBalancer != nil {
		cp.LoadBalancer = o.LoadBalancer.DeepCopy()
	}
	return &cp
}

// DeepCopy generates a deep copy of *DiscoveryResolver
func (o *DiscoveryResolver) DeepCopy() *DiscoveryResolver {
	var cp DiscoveryResolver = *o
	if o.Failover != nil {
		cp.Failover = o.Failover.DeepCopy()
	}
	if o.PrioritizeByLocality != nil {
		cp.PrioritizeByLocality = new(DiscoveryPrioritizeByLocality)
		*cp.PrioritizeByLocality = *o.PrioritizeByLocality
	}
	return &cp
}

// DeepCopy generates a deep copy of *DiscoveryRoute
func (o *DiscoveryRoute) DeepCopy() *DiscoveryRoute {
	var cp DiscoveryRoute = *o
	if o.Definition != nil {
		cp.Definition = o.Definition.DeepCopy()
	}
	return &cp
}

// DeepCopy generates a deep copy of *DiscoverySplit
func (o *DiscoverySplit) DeepCopy() *DiscoverySplit {
	var cp DiscoverySplit = *o
	if o.Definition != nil {
		cp.Definition = new(ServiceSplit)
		*cp.Definition = *o.Definition
		if o.Definition.RequestHeaders != nil {
			cp.Definition.RequestHeaders = o.Definition.RequestHeaders.DeepCopy()
		}
		if o.Definition.ResponseHeaders != nil {
			cp.Definition.ResponseHeaders = o.Definition.ResponseHeaders.DeepCopy()
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *ExposeConfig
func (o *ExposeConfig) DeepCopy() *ExposeConfig {
	var cp ExposeConfig = *o
	if o.Paths != nil {
		cp.Paths = make([]ExposePath, len(o.Paths))
		copy(cp.Paths, o.Paths)
	}
	return &cp
}

// DeepCopy generates a deep copy of *ExportedServicesConfigEntry
func (o *ExportedServicesConfigEntry) DeepCopy() *ExportedServicesConfigEntry {
	var cp ExportedServicesConfigEntry = *o
	if o.Services != nil {
		cp.Services = make([]ExportedService, len(o.Services))
		copy(cp.Services, o.Services)
		for i2 := range o.Services {
			if o.Services[i2].Consumers != nil {
				cp.Services[i2].Consumers = make([]ServiceConsumer, len(o.Services[i2].Consumers))
				copy(cp.Services[i2].Consumers, o.Services[i2].Consumers)
			}
		}
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *GatewayService
func (o *GatewayService) DeepCopy() *GatewayService {
	var cp GatewayService = *o
	if o.Hosts != nil {
		cp.Hosts = make([]string, len(o.Hosts))
		copy(cp.Hosts, o.Hosts)
	}
	return &cp
}

// DeepCopy generates a deep copy of *GatewayServiceTLSConfig
func (o *GatewayServiceTLSConfig) DeepCopy() *GatewayServiceTLSConfig {
	var cp GatewayServiceTLSConfig = *o
	if o.SDS != nil {
		cp.SDS = new(GatewayTLSSDSConfig)
		*cp.SDS = *o.SDS
	}
	return &cp
}

// DeepCopy generates a deep copy of *HTTPHeaderModifiers
func (o *HTTPHeaderModifiers) DeepCopy() *HTTPHeaderModifiers {
	var cp HTTPHeaderModifiers = *o
	if o.Add != nil {
		cp.Add = make(map[string]string, len(o.Add))
		for k2, v2 := range o.Add {
			cp.Add[k2] = v2
		}
	}
	if o.Set != nil {
		cp.Set = make(map[string]string, len(o.Set))
		for k2, v2 := range o.Set {
			cp.Set[k2] = v2
		}
	}
	if o.Remove != nil {
		cp.Remove = make([]string, len(o.Remove))
		copy(cp.Remove, o.Remove)
	}
	return &cp
}

// DeepCopy generates a deep copy of *HTTPRouteConfigEntry
func (o *HTTPRouteConfigEntry) DeepCopy() *HTTPRouteConfigEntry {
	var cp HTTPRouteConfigEntry = *o
	if o.Parents != nil {
		cp.Parents = make([]ResourceReference, len(o.Parents))
		copy(cp.Parents, o.Parents)
	}
	if o.Rules != nil {
		cp.Rules = make([]HTTPRouteRule, len(o.Rules))
		copy(cp.Rules, o.Rules)
		for i2 := range o.Rules {
			if o.Rules[i2].Filters.Headers != nil {
				cp.Rules[i2].Filters.Headers = make([]HTTPHeaderFilter, len(o.Rules[i2].Filters.Headers))
				copy(cp.Rules[i2].Filters.Headers, o.Rules[i2].Filters.Headers)
				for i5 := range o.Rules[i2].Filters.Headers {
					if o.Rules[i2].Filters.Headers[i5].Add != nil {
						cp.Rules[i2].Filters.Headers[i5].Add = make(map[string]string, len(o.Rules[i2].Filters.Headers[i5].Add))
						for k7, v7 := range o.Rules[i2].Filters.Headers[i5].Add {
							cp.Rules[i2].Filters.Headers[i5].Add[k7] = v7
						}
					}
					if o.Rules[i2].Filters.Headers[i5].Remove != nil {
						cp.Rules[i2].Filters.Headers[i5].Remove = make([]string, len(o.Rules[i2].Filters.Headers[i5].Remove))
						copy(cp.Rules[i2].Filters.Headers[i5].Remove, o.Rules[i2].Filters.Headers[i5].Remove)
					}
					if o.Rules[i2].Filters.Headers[i5].Set != nil {
						cp.Rules[i2].Filters.Headers[i5].Set = make(map[string]string, len(o.Rules[i2].Filters.Headers[i5].Set))
						for k7, v7 := range o.Rules[i2].Filters.Headers[i5].Set {
							cp.Rules[i2].Filters.Headers[i5].Set[k7] = v7
						}
					}
				}
			}
			if o.Rules[i2].Filters.URLRewrite != nil {
				cp.Rules[i2].Filters.URLRewrite = new(URLRewrite)
				*cp.Rules[i2].Filters.URLRewrite = *o.Rules[i2].Filters.URLRewrite
			}
			if o.Rules[i2].Matches != nil {
				cp.Rules[i2].Matches = make([]HTTPMatch, len(o.Rules[i2].Matches))
				copy(cp.Rules[i2].Matches, o.Rules[i2].Matches)
				for i4 := range o.Rules[i2].Matches {
					if o.Rules[i2].Matches[i4].Headers != nil {
						cp.Rules[i2].Matches[i4].Headers = make([]HTTPHeaderMatch, len(o.Rules[i2].Matches[i4].Headers))
						copy(cp.Rules[i2].Matches[i4].Headers, o.Rules[i2].Matches[i4].Headers)
					}
					if o.Rules[i2].Matches[i4].Query != nil {
						cp.Rules[i2].Matches[i4].Query = make([]HTTPQueryMatch, len(o.Rules[i2].Matches[i4].Query))
						copy(cp.Rules[i2].Matches[i4].Query, o.Rules[i2].Matches[i4].Query)
					}
				}
			}
			if o.Rules[i2].Services != nil {
				cp.Rules[i2].Services = make([]HTTPService, len(o.Rules[i2].Services))
				copy(cp.Rules[i2].Services, o.Rules[i2].Services)
				for i4 := range o.Rules[i2].Services {
					if o.Rules[i2].Services[i4].Filters.Headers != nil {
						cp.Rules[i2].Services[i4].Filters.Headers = make([]HTTPHeaderFilter, len(o.Rules[i2].Services[i4].Filters.Headers))
						copy(cp.Rules[i2].Services[i4].Filters.Headers, o.Rules[i2].Services[i4].Filters.Headers)
						for i7 := range o.Rules[i2].Services[i4].Filters.Headers {
							if o.Rules[i2].Services[i4].Filters.Headers[i7].Add != nil {
								cp.Rules[i2].Services[i4].Filters.Headers[i7].Add = make(map[string]string, len(o.Rules[i2].Services[i4].Filters.Headers[i7].Add))
								for k9, v9 := range o.Rules[i2].Services[i4].Filters.Headers[i7].Add {
									cp.Rules[i2].Services[i4].Filters.Headers[i7].Add[k9] = v9
								}
							}
							if o.Rules[i2].Services[i4].Filters.Headers[i7].Remove != nil {
								cp.Rules[i2].Services[i4].Filters.Headers[i7].Remove = make([]string, len(o.Rules[i2].Services[i4].Filters.Headers[i7].Remove))
								copy(cp.Rules[i2].Services[i4].Filters.Headers[i7].Remove, o.Rules[i2].Services[i4].Filters.Headers[i7].Remove)
							}
							if o.Rules[i2].Services[i4].Filters.Headers[i7].Set != nil {
								cp.Rules[i2].Services[i4].Filters.Headers[i7].Set = make(map[string]string, len(o.Rules[i2].Services[i4].Filters.Headers[i7].Set))
								for k9, v9 := range o.Rules[i2].Services[i4].Filters.Headers[i7].Set {
									cp.Rules[i2].Services[i4].Filters.Headers[i7].Set[k9] = v9
								}
							}
						}
					}
					if o.Rules[i2].Services[i4].Filters.URLRewrite != nil {
						cp.Rules[i2].Services[i4].Filters.URLRewrite = new(URLRewrite)
						*cp.Rules[i2].Services[i4].Filters.URLRewrite = *o.Rules[i2].Services[i4].Filters.URLRewrite
					}
				}
			}
		}
	}
	if o.Hostnames != nil {
		cp.Hostnames = make([]string, len(o.Hostnames))
		copy(cp.Hostnames, o.Hostnames)
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	{
		retV := o.Status.DeepCopy()
		cp.Status = *retV
	}
	return &cp
}

// DeepCopy generates a deep copy of *HashPolicy
func (o *HashPolicy) DeepCopy() *HashPolicy {
	var cp HashPolicy = *o
	if o.CookieConfig != nil {
		cp.CookieConfig = new(CookieConfig)
		*cp.CookieConfig = *o.CookieConfig
	}
	return &cp
}

// DeepCopy generates a deep copy of *HealthCheck
func (o *HealthCheck) DeepCopy() *HealthCheck {
	var cp HealthCheck = *o
	if o.ServiceTags != nil {
		cp.ServiceTags = make([]string, len(o.ServiceTags))
		copy(cp.ServiceTags, o.ServiceTags)
	}
	if o.Definition.Header != nil {
		cp.Definition.Header = make(map[string][]string, len(o.Definition.Header))
		for k3, v3 := range o.Definition.Header {
			var cp_Definition_Header_v3 []string
			if v3 != nil {
				cp_Definition_Header_v3 = make([]string, len(v3))
				copy(cp_Definition_Header_v3, v3)
			}
			cp.Definition.Header[k3] = cp_Definition_Header_v3
		}
	}
	if o.Definition.ScriptArgs != nil {
		cp.Definition.ScriptArgs = make([]string, len(o.Definition.ScriptArgs))
		copy(cp.Definition.ScriptArgs, o.Definition.ScriptArgs)
	}
	return &cp
}

// DeepCopy generates a deep copy of *IndexedCARoots
func (o *IndexedCARoots) DeepCopy() *IndexedCARoots {
	var cp IndexedCARoots = *o
	if o.Roots != nil {
		cp.Roots = make([]*CARoot, len(o.Roots))
		copy(cp.Roots, o.Roots)
		for i2 := range o.Roots {
			if o.Roots[i2] != nil {
				cp.Roots[i2] = o.Roots[i2].DeepCopy()
			}
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *IngressListener
func (o *IngressListener) DeepCopy() *IngressListener {
	var cp IngressListener = *o
	if o.TLS != nil {
		cp.TLS = new(GatewayTLSConfig)
		*cp.TLS = *o.TLS
		if o.TLS.SDS != nil {
			cp.TLS.SDS = new(GatewayTLSSDSConfig)
			*cp.TLS.SDS = *o.TLS.SDS
		}
		if o.TLS.CipherSuites != nil {
			cp.TLS.CipherSuites = make([]types.TLSCipherSuite, len(o.TLS.CipherSuites))
			copy(cp.TLS.CipherSuites, o.TLS.CipherSuites)
		}
	}
	if o.Services != nil {
		cp.Services = make([]IngressService, len(o.Services))
		copy(cp.Services, o.Services)
		for i2 := range o.Services {
			if o.Services[i2].Hosts != nil {
				cp.Services[i2].Hosts = make([]string, len(o.Services[i2].Hosts))
				copy(cp.Services[i2].Hosts, o.Services[i2].Hosts)
			}
			if o.Services[i2].TLS != nil {
				cp.Services[i2].TLS = o.Services[i2].TLS.DeepCopy()
			}
			if o.Services[i2].RequestHeaders != nil {
				cp.Services[i2].RequestHeaders = o.Services[i2].RequestHeaders.DeepCopy()
			}
			if o.Services[i2].ResponseHeaders != nil {
				cp.Services[i2].ResponseHeaders = o.Services[i2].ResponseHeaders.DeepCopy()
			}
			if o.Services[i2].PassiveHealthCheck != nil {
				cp.Services[i2].PassiveHealthCheck = new(PassiveHealthCheck)
				*cp.Services[i2].PassiveHealthCheck = *o.Services[i2].PassiveHealthCheck
				if o.Services[i2].PassiveHealthCheck.EnforcingConsecutive5xx != nil {
					cp.Services[i2].PassiveHealthCheck.EnforcingConsecutive5xx = new(uint32)
					*cp.Services[i2].PassiveHealthCheck.EnforcingConsecutive5xx = *o.Services[i2].PassiveHealthCheck.EnforcingConsecutive5xx
				}
			}
			if o.Services[i2].Meta != nil {
				cp.Services[i2].Meta = make(map[string]string, len(o.Services[i2].Meta))
				for k4, v4 := range o.Services[i2].Meta {
					cp.Services[i2].Meta[k4] = v4
				}
			}
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *InlineCertificateConfigEntry
func (o *InlineCertificateConfigEntry) DeepCopy() *InlineCertificateConfigEntry {
	var cp InlineCertificateConfigEntry = *o
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *Intention
func (o *Intention) DeepCopy() *Intention {
	var cp Intention = *o
	if o.Permissions != nil {
		cp.Permissions = make([]*IntentionPermission, len(o.Permissions))
		copy(cp.Permissions, o.Permissions)
		for i2 := range o.Permissions {
			if o.Permissions[i2] != nil {
				cp.Permissions[i2] = o.Permissions[i2].DeepCopy()
			}
		}
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	if o.Hash != nil {
		cp.Hash = make([]byte, len(o.Hash))
		copy(cp.Hash, o.Hash)
	}
	return &cp
}

// DeepCopy generates a deep copy of *IntentionPermission
func (o *IntentionPermission) DeepCopy() *IntentionPermission {
	var cp IntentionPermission = *o
	if o.HTTP != nil {
		cp.HTTP = new(IntentionHTTPPermission)
		*cp.HTTP = *o.HTTP
		if o.HTTP.Header != nil {
			cp.HTTP.Header = make([]IntentionHTTPHeaderPermission, len(o.HTTP.Header))
			copy(cp.HTTP.Header, o.HTTP.Header)
		}
		if o.HTTP.Methods != nil {
			cp.HTTP.Methods = make([]string, len(o.HTTP.Methods))
			copy(cp.HTTP.Methods, o.HTTP.Methods)
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *LoadBalancer
func (o *LoadBalancer) DeepCopy() *LoadBalancer {
	var cp LoadBalancer = *o
	if o.RingHashConfig != nil {
		cp.RingHashConfig = new(RingHashConfig)
		*cp.RingHashConfig = *o.RingHashConfig
	}
	if o.LeastRequestConfig != nil {
		cp.LeastRequestConfig = new(LeastRequestConfig)
		*cp.LeastRequestConfig = *o.LeastRequestConfig
	}
	if o.HashPolicies != nil {
		cp.HashPolicies = make([]HashPolicy, len(o.HashPolicies))
		copy(cp.HashPolicies, o.HashPolicies)
		for i2 := range o.HashPolicies {
			{
				retV := o.HashPolicies[i2].DeepCopy()
				cp.HashPolicies[i2] = *retV
			}
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *MeshConfigEntry
func (o *MeshConfigEntry) DeepCopy() *MeshConfigEntry {
	var cp MeshConfigEntry = *o
	if o.TLS != nil {
		cp.TLS = o.TLS.DeepCopy()
	}
	if o.HTTP != nil {
		cp.HTTP = new(MeshHTTPConfig)
		*cp.HTTP = *o.HTTP
	}
	if o.Peering != nil {
		cp.Peering = new(PeeringMeshConfig)
		*cp.Peering = *o.Peering
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *MeshDirectionalTLSConfig
func (o *MeshDirectionalTLSConfig) DeepCopy() *MeshDirectionalTLSConfig {
	var cp MeshDirectionalTLSConfig = *o
	if o.CipherSuites != nil {
		cp.CipherSuites = make([]types.TLSCipherSuite, len(o.CipherSuites))
		copy(cp.CipherSuites, o.CipherSuites)
	}
	return &cp
}

// DeepCopy generates a deep copy of *MeshTLSConfig
func (o *MeshTLSConfig) DeepCopy() *MeshTLSConfig {
	var cp MeshTLSConfig = *o
	if o.Incoming != nil {
		cp.Incoming = o.Incoming.DeepCopy()
	}
	if o.Outgoing != nil {
		cp.Outgoing = o.Outgoing.DeepCopy()
	}
	return &cp
}

// DeepCopy generates a deep copy of *Node
func (o *Node) DeepCopy() *Node {
	var cp Node = *o
	if o.TaggedAddresses != nil {
		cp.TaggedAddresses = make(map[string]string, len(o.TaggedAddresses))
		for k2, v2 := range o.TaggedAddresses {
			cp.TaggedAddresses[k2] = v2
		}
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	if o.Locality != nil {
		cp.Locality = new(Locality)
		*cp.Locality = *o.Locality
	}
	return &cp
}

// DeepCopy generates a deep copy of *NodeService
func (o *NodeService) DeepCopy() *NodeService {
	var cp NodeService = *o
	if o.Tags != nil {
		cp.Tags = make([]string, len(o.Tags))
		copy(cp.Tags, o.Tags)
	}
	if o.TaggedAddresses != nil {
		cp.TaggedAddresses = make(map[string]ServiceAddress, len(o.TaggedAddresses))
		for k2, v2 := range o.TaggedAddresses {
			cp.TaggedAddresses[k2] = v2
		}
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	if o.Weights != nil {
		cp.Weights = new(Weights)
		*cp.Weights = *o.Weights
	}
	if o.Locality != nil {
		cp.Locality = new(Locality)
		*cp.Locality = *o.Locality
	}
	{
		retV := o.Proxy.DeepCopy()
		cp.Proxy = *retV
	}
	{
		retV := o.Connect.DeepCopy()
		cp.Connect = *retV
	}
	return &cp
}

// DeepCopy generates a deep copy of *PeeringServiceMeta
func (o *PeeringServiceMeta) DeepCopy() *PeeringServiceMeta {
	var cp PeeringServiceMeta = *o
	if o.SNI != nil {
		cp.SNI = make([]string, len(o.SNI))
		copy(cp.SNI, o.SNI)
	}
	if o.SpiffeID != nil {
		cp.SpiffeID = make([]string, len(o.SpiffeID))
		copy(cp.SpiffeID, o.SpiffeID)
	}
	return &cp
}

// DeepCopy generates a deep copy of *ServiceConfigEntry
func (o *ServiceConfigEntry) DeepCopy() *ServiceConfigEntry {
	var cp ServiceConfigEntry = *o
	{
		retV := o.Expose.DeepCopy()
		cp.Expose = *retV
	}
	if o.UpstreamConfig != nil {
		cp.UpstreamConfig = o.UpstreamConfig.DeepCopy()
	}
	if o.Destination != nil {
		cp.Destination = new(DestinationConfig)
		*cp.Destination = *o.Destination
		if o.Destination.Addresses != nil {
			cp.Destination.Addresses = make([]string, len(o.Destination.Addresses))
			copy(cp.Destination.Addresses, o.Destination.Addresses)
		}
	}
	if o.EnvoyExtensions != nil {
		cp.EnvoyExtensions = make([]EnvoyExtension, len(o.EnvoyExtensions))
		copy(cp.EnvoyExtensions, o.EnvoyExtensions)
		for i2 := range o.EnvoyExtensions {
			if o.EnvoyExtensions[i2].Arguments != nil {
				cp.EnvoyExtensions[i2].Arguments = make(map[string]interface{}, len(o.EnvoyExtensions[i2].Arguments))
				for k4, v4 := range o.EnvoyExtensions[i2].Arguments {
					cp.EnvoyExtensions[i2].Arguments[k4] = v4
				}
			}
		}
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *ServiceConfigResponse
func (o *ServiceConfigResponse) DeepCopy() *ServiceConfigResponse {
	var cp ServiceConfigResponse = *o
	if o.ProxyConfig != nil {
		cp.ProxyConfig = make(map[string]interface{}, len(o.ProxyConfig))
		for k2, v2 := range o.ProxyConfig {
			cp.ProxyConfig[k2] = v2
		}
	}
	if o.UpstreamConfigs != nil {
		cp.UpstreamConfigs = make([]OpaqueUpstreamConfig, len(o.UpstreamConfigs))
		copy(cp.UpstreamConfigs, o.UpstreamConfigs)
		for i2 := range o.UpstreamConfigs {
			if o.UpstreamConfigs[i2].Config != nil {
				cp.UpstreamConfigs[i2].Config = make(map[string]interface{}, len(o.UpstreamConfigs[i2].Config))
				for k4, v4 := range o.UpstreamConfigs[i2].Config {
					cp.UpstreamConfigs[i2].Config[k4] = v4
				}
			}
		}
	}
	{
		retV := o.Expose.DeepCopy()
		cp.Expose = *retV
	}
	if o.Destination.Addresses != nil {
		cp.Destination.Addresses = make([]string, len(o.Destination.Addresses))
		copy(cp.Destination.Addresses, o.Destination.Addresses)
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	if o.EnvoyExtensions != nil {
		cp.EnvoyExtensions = make([]EnvoyExtension, len(o.EnvoyExtensions))
		copy(cp.EnvoyExtensions, o.EnvoyExtensions)
		for i2 := range o.EnvoyExtensions {
			if o.EnvoyExtensions[i2].Arguments != nil {
				cp.EnvoyExtensions[i2].Arguments = make(map[string]interface{}, len(o.EnvoyExtensions[i2].Arguments))
				for k4, v4 := range o.EnvoyExtensions[i2].Arguments {
					cp.EnvoyExtensions[i2].Arguments[k4] = v4
				}
			}
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *ServiceConnect
func (o *ServiceConnect) DeepCopy() *ServiceConnect {
	var cp ServiceConnect = *o
	if o.SidecarService != nil {
		cp.SidecarService = o.SidecarService.DeepCopy()
	}
	if o.PeerMeta != nil {
		cp.PeerMeta = o.PeerMeta.DeepCopy()
	}
	return &cp
}

// DeepCopy generates a deep copy of *ServiceDefinition
func (o *ServiceDefinition) DeepCopy() *ServiceDefinition {
	var cp ServiceDefinition = *o
	if o.Tags != nil {
		cp.Tags = make([]string, len(o.Tags))
		copy(cp.Tags, o.Tags)
	}
	if o.TaggedAddresses != nil {
		cp.TaggedAddresses = make(map[string]ServiceAddress, len(o.TaggedAddresses))
		for k2, v2 := range o.TaggedAddresses {
			cp.TaggedAddresses[k2] = v2
		}
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	{
		retV := o.Check.DeepCopy()
		cp.Check = *retV
	}
	if o.Checks != nil {
		cp.Checks = make([]*CheckType, len(o.Checks))
		copy(cp.Checks, o.Checks)
		for i2 := range o.Checks {
			if o.Checks[i2] != nil {
				cp.Checks[i2] = o.Checks[i2].DeepCopy()
			}
		}
	}
	if o.Weights != nil {
		cp.Weights = new(Weights)
		*cp.Weights = *o.Weights
	}
	if o.Locality != nil {
		cp.Locality = new(Locality)
		*cp.Locality = *o.Locality
	}
	if o.Proxy != nil {
		cp.Proxy = o.Proxy.DeepCopy()
	}
	if o.Connect != nil {
		cp.Connect = o.Connect.DeepCopy()
	}
	return &cp
}

// DeepCopy generates a deep copy of *ServiceResolverConfigEntry
func (o *ServiceResolverConfigEntry) DeepCopy() *ServiceResolverConfigEntry {
	var cp ServiceResolverConfigEntry = *o
	if o.Subsets != nil {
		cp.Subsets = make(map[string]ServiceResolverSubset, len(o.Subsets))
		for k2, v2 := range o.Subsets {
			cp.Subsets[k2] = v2
		}
	}
	if o.Redirect != nil {
		cp.Redirect = new(ServiceResolverRedirect)
		*cp.Redirect = *o.Redirect
	}
	if o.Failover != nil {
		cp.Failover = make(map[string]ServiceResolverFailover, len(o.Failover))
		for k2, v2 := range o.Failover {
			var cp_Failover_v2 ServiceResolverFailover
			{
				retV := v2.DeepCopy()
				cp_Failover_v2 = *retV
			}
			cp.Failover[k2] = cp_Failover_v2
		}
	}
	if o.PrioritizeByLocality != nil {
		cp.PrioritizeByLocality = new(ServiceResolverPrioritizeByLocality)
		*cp.PrioritizeByLocality = *o.PrioritizeByLocality
	}
	if o.LoadBalancer != nil {
		cp.LoadBalancer = o.LoadBalancer.DeepCopy()
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *ServiceResolverFailover
func (o *ServiceResolverFailover) DeepCopy() *ServiceResolverFailover {
	var cp ServiceResolverFailover = *o
	if o.Datacenters != nil {
		cp.Datacenters = make([]string, len(o.Datacenters))
		copy(cp.Datacenters, o.Datacenters)
	}
	if o.Targets != nil {
		cp.Targets = make([]ServiceResolverFailoverTarget, len(o.Targets))
		copy(cp.Targets, o.Targets)
	}
	if o.Policy != nil {
		cp.Policy = new(ServiceResolverFailoverPolicy)
		*cp.Policy = *o.Policy
		if o.Policy.Regions != nil {
			cp.Policy.Regions = make([]string, len(o.Policy.Regions))
			copy(cp.Policy.Regions, o.Policy.Regions)
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *ServiceRoute
func (o *ServiceRoute) DeepCopy() *ServiceRoute {
	var cp ServiceRoute = *o
	if o.Match != nil {
		cp.Match = o.Match.DeepCopy()
	}
	if o.Destination != nil {
		cp.Destination = o.Destination.DeepCopy()
	}
	return &cp
}

// DeepCopy generates a deep copy of *ServiceRouteDestination
func (o *ServiceRouteDestination) DeepCopy() *ServiceRouteDestination {
	var cp ServiceRouteDestination = *o
	if o.RetryOn != nil {
		cp.RetryOn = make([]string, len(o.RetryOn))
		copy(cp.RetryOn, o.RetryOn)
	}
	if o.RetryOnStatusCodes != nil {
		cp.RetryOnStatusCodes = make([]uint32, len(o.RetryOnStatusCodes))
		copy(cp.RetryOnStatusCodes, o.RetryOnStatusCodes)
	}
	if o.RequestHeaders != nil {
		cp.RequestHeaders = o.RequestHeaders.DeepCopy()
	}
	if o.ResponseHeaders != nil {
		cp.ResponseHeaders = o.ResponseHeaders.DeepCopy()
	}
	return &cp
}

// DeepCopy generates a deep copy of *ServiceRouteMatch
func (o *ServiceRouteMatch) DeepCopy() *ServiceRouteMatch {
	var cp ServiceRouteMatch = *o
	if o.HTTP != nil {
		cp.HTTP = new(ServiceRouteHTTPMatch)
		*cp.HTTP = *o.HTTP
		if o.HTTP.Header != nil {
			cp.HTTP.Header = make([]ServiceRouteHTTPMatchHeader, len(o.HTTP.Header))
			copy(cp.HTTP.Header, o.HTTP.Header)
		}
		if o.HTTP.QueryParam != nil {
			cp.HTTP.QueryParam = make([]ServiceRouteHTTPMatchQueryParam, len(o.HTTP.QueryParam))
			copy(cp.HTTP.QueryParam, o.HTTP.QueryParam)
		}
		if o.HTTP.Methods != nil {
			cp.HTTP.Methods = make([]string, len(o.HTTP.Methods))
			copy(cp.HTTP.Methods, o.HTTP.Methods)
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *TCPRouteConfigEntry
func (o *TCPRouteConfigEntry) DeepCopy() *TCPRouteConfigEntry {
	var cp TCPRouteConfigEntry = *o
	if o.Parents != nil {
		cp.Parents = make([]ResourceReference, len(o.Parents))
		copy(cp.Parents, o.Parents)
	}
	if o.Services != nil {
		cp.Services = make([]TCPService, len(o.Services))
		copy(cp.Services, o.Services)
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	{
		retV := o.Status.DeepCopy()
		cp.Status = *retV
	}
	return &cp
}

// DeepCopy generates a deep copy of *Upstream
func (o *Upstream) DeepCopy() *Upstream {
	var cp Upstream = *o
	if o.Config != nil {
		cp.Config = make(map[string]interface{}, len(o.Config))
		for k2, v2 := range o.Config {
			cp.Config[k2] = v2
		}
	}
	if o.IngressHosts != nil {
		cp.IngressHosts = make([]string, len(o.IngressHosts))
		copy(cp.IngressHosts, o.IngressHosts)
	}
	return &cp
}

// DeepCopy generates a deep copy of *UpstreamConfiguration
func (o *UpstreamConfiguration) DeepCopy() *UpstreamConfiguration {
	var cp UpstreamConfiguration = *o
	if o.Overrides != nil {
		cp.Overrides = make([]*UpstreamConfig, len(o.Overrides))
		copy(cp.Overrides, o.Overrides)
		for i2 := range o.Overrides {
			if o.Overrides[i2] != nil {
				cp.Overrides[i2] = new(UpstreamConfig)
				*cp.Overrides[i2] = *o.Overrides[i2]
				if o.Overrides[i2].Limits != nil {
					cp.Overrides[i2].Limits = new(UpstreamLimits)
					*cp.Overrides[i2].Limits = *o.Overrides[i2].Limits
					if o.Overrides[i2].Limits.MaxConnections != nil {
						cp.Overrides[i2].Limits.MaxConnections = new(int)
						*cp.Overrides[i2].Limits.MaxConnections = *o.Overrides[i2].Limits.MaxConnections
					}
					if o.Overrides[i2].Limits.MaxPendingRequests != nil {
						cp.Overrides[i2].Limits.MaxPendingRequests = new(int)
						*cp.Overrides[i2].Limits.MaxPendingRequests = *o.Overrides[i2].Limits.MaxPendingRequests
					}
					if o.Overrides[i2].Limits.MaxConcurrentRequests != nil {
						cp.Overrides[i2].Limits.MaxConcurrentRequests = new(int)
						*cp.Overrides[i2].Limits.MaxConcurrentRequests = *o.Overrides[i2].Limits.MaxConcurrentRequests
					}
				}
				if o.Overrides[i2].PassiveHealthCheck != nil {
					cp.Overrides[i2].PassiveHealthCheck = new(PassiveHealthCheck)
					*cp.Overrides[i2].PassiveHealthCheck = *o.Overrides[i2].PassiveHealthCheck
					if o.Overrides[i2].PassiveHealthCheck.EnforcingConsecutive5xx != nil {
						cp.Overrides[i2].PassiveHealthCheck.EnforcingConsecutive5xx = new(uint32)
						*cp.Overrides[i2].PassiveHealthCheck.EnforcingConsecutive5xx = *o.Overrides[i2].PassiveHealthCheck.EnforcingConsecutive5xx
					}
				}
			}
		}
	}
	if o.Defaults != nil {
		cp.Defaults = new(UpstreamConfig)
		*cp.Defaults = *o.Defaults
		if o.Defaults.Limits != nil {
			cp.Defaults.Limits = new(UpstreamLimits)
			*cp.Defaults.Limits = *o.Defaults.Limits
			if o.Defaults.Limits.MaxConnections != nil {
				cp.Defaults.Limits.MaxConnections = new(int)
				*cp.Defaults.Limits.MaxConnections = *o.Defaults.Limits.MaxConnections
			}
			if o.Defaults.Limits.MaxPendingRequests != nil {
				cp.Defaults.Limits.MaxPendingRequests = new(int)
				*cp.Defaults.Limits.MaxPendingRequests = *o.Defaults.Limits.MaxPendingRequests
			}
			if o.Defaults.Limits.MaxConcurrentRequests != nil {
				cp.Defaults.Limits.MaxConcurrentRequests = new(int)
				*cp.Defaults.Limits.MaxConcurrentRequests = *o.Defaults.Limits.MaxConcurrentRequests
			}
		}
		if o.Defaults.PassiveHealthCheck != nil {
			cp.Defaults.PassiveHealthCheck = new(PassiveHealthCheck)
			*cp.Defaults.PassiveHealthCheck = *o.Defaults.PassiveHealthCheck
			if o.Defaults.PassiveHealthCheck.EnforcingConsecutive5xx != nil {
				cp.Defaults.PassiveHealthCheck.EnforcingConsecutive5xx = new(uint32)
				*cp.Defaults.PassiveHealthCheck.EnforcingConsecutive5xx = *o.Defaults.PassiveHealthCheck.EnforcingConsecutive5xx
			}
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *Status
func (o *Status) DeepCopy() *Status {
	var cp Status = *o
	if o.Conditions != nil {
		cp.Conditions = make([]Condition, len(o.Conditions))
		copy(cp.Conditions, o.Conditions)
		for i2 := range o.Conditions {
			if o.Conditions[i2].Resource != nil {
				cp.Conditions[i2].Resource = new(ResourceReference)
				*cp.Conditions[i2].Resource = *o.Conditions[i2].Resource
			}
			if o.Conditions[i2].LastTransitionTime != nil {
				cp.Conditions[i2].LastTransitionTime = new(time.Time)
				*cp.Conditions[i2].LastTransitionTime = *o.Conditions[i2].LastTransitionTime
			}
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *BoundAPIGatewayConfigEntry
func (o *BoundAPIGatewayConfigEntry) DeepCopy() *BoundAPIGatewayConfigEntry {
	var cp BoundAPIGatewayConfigEntry = *o
	if o.Listeners != nil {
		cp.Listeners = make([]BoundAPIGatewayListener, len(o.Listeners))
		copy(cp.Listeners, o.Listeners)
		for i2 := range o.Listeners {
			{
				retV := o.Listeners[i2].DeepCopy()
				cp.Listeners[i2] = *retV
			}
		}
	}
	if o.Meta != nil {
		cp.Meta = make(map[string]string, len(o.Meta))
		for k2, v2 := range o.Meta {
			cp.Meta[k2] = v2
		}
	}
	return &cp
}
