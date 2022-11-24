// Code generated by mog. DO NOT EDIT.

package pbconfigentry

import "github.com/hashicorp/consul/agent/structs"

func CookieConfigToStructs(s *CookieConfig, t *structs.CookieConfig) {
	if s == nil {
		return
	}
	t.Session = s.Session
	t.TTL = structs.DurationFromProto(s.TTL)
	t.Path = s.Path
}
func CookieConfigFromStructs(t *structs.CookieConfig, s *CookieConfig) {
	if s == nil {
		return
	}
	s.Session = t.Session
	s.TTL = structs.DurationToProto(t.TTL)
	s.Path = t.Path
}
func DestinationConfigToStructs(s *DestinationConfig, t *structs.DestinationConfig) {
	if s == nil {
		return
	}
	t.Addresses = s.Addresses
	t.Port = int(s.Port)
}
func DestinationConfigFromStructs(t *structs.DestinationConfig, s *DestinationConfig) {
	if s == nil {
		return
	}
	s.Addresses = t.Addresses
	s.Port = int32(t.Port)
}
func EnvoyExtensionToStructs(s *EnvoyExtension, t *structs.EnvoyExtension) {
	if s == nil {
		return
	}
	t.Name = s.Name
	t.Required = s.Required
	t.Arguments = envoyExtensionArgumentsToStructs(s.Arguments)
}
func EnvoyExtensionFromStructs(t *structs.EnvoyExtension, s *EnvoyExtension) {
	if s == nil {
		return
	}
	s.Name = t.Name
	s.Required = t.Required
	s.Arguments = envoyExtensionArgumentsFromStructs(t.Arguments)
}
func ExposeConfigToStructs(s *ExposeConfig, t *structs.ExposeConfig) {
	if s == nil {
		return
	}
	t.Checks = s.Checks
	{
		t.Paths = make([]structs.ExposePath, len(s.Paths))
		for i := range s.Paths {
			if s.Paths[i] != nil {
				ExposePathToStructs(s.Paths[i], &t.Paths[i])
			}
		}
	}
}
func ExposeConfigFromStructs(t *structs.ExposeConfig, s *ExposeConfig) {
	if s == nil {
		return
	}
	s.Checks = t.Checks
	{
		s.Paths = make([]*ExposePath, len(t.Paths))
		for i := range t.Paths {
			{
				var x ExposePath
				ExposePathFromStructs(&t.Paths[i], &x)
				s.Paths[i] = &x
			}
		}
	}
}
func ExposePathToStructs(s *ExposePath, t *structs.ExposePath) {
	if s == nil {
		return
	}
	t.ListenerPort = int(s.ListenerPort)
	t.Path = s.Path
	t.LocalPathPort = int(s.LocalPathPort)
	t.Protocol = s.Protocol
	t.ParsedFromCheck = s.ParsedFromCheck
}
func ExposePathFromStructs(t *structs.ExposePath, s *ExposePath) {
	if s == nil {
		return
	}
	s.ListenerPort = int32(t.ListenerPort)
	s.Path = t.Path
	s.LocalPathPort = int32(t.LocalPathPort)
	s.Protocol = t.Protocol
	s.ParsedFromCheck = t.ParsedFromCheck
}
func GatewayServiceTLSConfigToStructs(s *GatewayServiceTLSConfig, t *structs.GatewayServiceTLSConfig) {
	if s == nil {
		return
	}
	if s.SDS != nil {
		var x structs.GatewayTLSSDSConfig
		GatewayTLSSDSConfigToStructs(s.SDS, &x)
		t.SDS = &x
	}
}
func GatewayServiceTLSConfigFromStructs(t *structs.GatewayServiceTLSConfig, s *GatewayServiceTLSConfig) {
	if s == nil {
		return
	}
	if t.SDS != nil {
		var x GatewayTLSSDSConfig
		GatewayTLSSDSConfigFromStructs(t.SDS, &x)
		s.SDS = &x
	}
}
func GatewayTLSConfigToStructs(s *GatewayTLSConfig, t *structs.GatewayTLSConfig) {
	if s == nil {
		return
	}
	t.Enabled = s.Enabled
	if s.SDS != nil {
		var x structs.GatewayTLSSDSConfig
		GatewayTLSSDSConfigToStructs(s.SDS, &x)
		t.SDS = &x
	}
	t.TLSMinVersion = tlsVersionToStructs(s.TLSMinVersion)
	t.TLSMaxVersion = tlsVersionToStructs(s.TLSMaxVersion)
	t.CipherSuites = cipherSuitesToStructs(s.CipherSuites)
}
func GatewayTLSConfigFromStructs(t *structs.GatewayTLSConfig, s *GatewayTLSConfig) {
	if s == nil {
		return
	}
	s.Enabled = t.Enabled
	if t.SDS != nil {
		var x GatewayTLSSDSConfig
		GatewayTLSSDSConfigFromStructs(t.SDS, &x)
		s.SDS = &x
	}
	s.TLSMinVersion = tlsVersionFromStructs(t.TLSMinVersion)
	s.TLSMaxVersion = tlsVersionFromStructs(t.TLSMaxVersion)
	s.CipherSuites = cipherSuitesFromStructs(t.CipherSuites)
}
func GatewayTLSSDSConfigToStructs(s *GatewayTLSSDSConfig, t *structs.GatewayTLSSDSConfig) {
	if s == nil {
		return
	}
	t.ClusterName = s.ClusterName
	t.CertResource = s.CertResource
}
func GatewayTLSSDSConfigFromStructs(t *structs.GatewayTLSSDSConfig, s *GatewayTLSSDSConfig) {
	if s == nil {
		return
	}
	s.ClusterName = t.ClusterName
	s.CertResource = t.CertResource
}
func HTTPHeaderModifiersToStructs(s *HTTPHeaderModifiers, t *structs.HTTPHeaderModifiers) {
	if s == nil {
		return
	}
	t.Add = s.Add
	t.Set = s.Set
	t.Remove = s.Remove
}
func HTTPHeaderModifiersFromStructs(t *structs.HTTPHeaderModifiers, s *HTTPHeaderModifiers) {
	if s == nil {
		return
	}
	s.Add = t.Add
	s.Set = t.Set
	s.Remove = t.Remove
}
func HashPolicyToStructs(s *HashPolicy, t *structs.HashPolicy) {
	if s == nil {
		return
	}
	t.Field = s.Field
	t.FieldValue = s.FieldValue
	if s.CookieConfig != nil {
		var x structs.CookieConfig
		CookieConfigToStructs(s.CookieConfig, &x)
		t.CookieConfig = &x
	}
	t.SourceIP = s.SourceIP
	t.Terminal = s.Terminal
}
func HashPolicyFromStructs(t *structs.HashPolicy, s *HashPolicy) {
	if s == nil {
		return
	}
	s.Field = t.Field
	s.FieldValue = t.FieldValue
	if t.CookieConfig != nil {
		var x CookieConfig
		CookieConfigFromStructs(t.CookieConfig, &x)
		s.CookieConfig = &x
	}
	s.SourceIP = t.SourceIP
	s.Terminal = t.Terminal
}
func IngressGatewayToStructs(s *IngressGateway, t *structs.IngressGatewayConfigEntry) {
	if s == nil {
		return
	}
	if s.TLS != nil {
		GatewayTLSConfigToStructs(s.TLS, &t.TLS)
	}
	{
		t.Listeners = make([]structs.IngressListener, len(s.Listeners))
		for i := range s.Listeners {
			if s.Listeners[i] != nil {
				IngressListenerToStructs(s.Listeners[i], &t.Listeners[i])
			}
		}
	}
	if s.Defaults != nil {
		var x structs.IngressServiceConfig
		IngressServiceConfigToStructs(s.Defaults, &x)
		t.Defaults = &x
	}
	t.Meta = s.Meta
}
func IngressGatewayFromStructs(t *structs.IngressGatewayConfigEntry, s *IngressGateway) {
	if s == nil {
		return
	}
	{
		var x GatewayTLSConfig
		GatewayTLSConfigFromStructs(&t.TLS, &x)
		s.TLS = &x
	}
	{
		s.Listeners = make([]*IngressListener, len(t.Listeners))
		for i := range t.Listeners {
			{
				var x IngressListener
				IngressListenerFromStructs(&t.Listeners[i], &x)
				s.Listeners[i] = &x
			}
		}
	}
	if t.Defaults != nil {
		var x IngressServiceConfig
		IngressServiceConfigFromStructs(t.Defaults, &x)
		s.Defaults = &x
	}
	s.Meta = t.Meta
}
func IngressListenerToStructs(s *IngressListener, t *structs.IngressListener) {
	if s == nil {
		return
	}
	t.Port = int(s.Port)
	t.Protocol = s.Protocol
	if s.TLS != nil {
		var x structs.GatewayTLSConfig
		GatewayTLSConfigToStructs(s.TLS, &x)
		t.TLS = &x
	}
	{
		t.Services = make([]structs.IngressService, len(s.Services))
		for i := range s.Services {
			if s.Services[i] != nil {
				IngressServiceToStructs(s.Services[i], &t.Services[i])
			}
		}
	}
}
func IngressListenerFromStructs(t *structs.IngressListener, s *IngressListener) {
	if s == nil {
		return
	}
	s.Port = int32(t.Port)
	s.Protocol = t.Protocol
	if t.TLS != nil {
		var x GatewayTLSConfig
		GatewayTLSConfigFromStructs(t.TLS, &x)
		s.TLS = &x
	}
	{
		s.Services = make([]*IngressService, len(t.Services))
		for i := range t.Services {
			{
				var x IngressService
				IngressServiceFromStructs(&t.Services[i], &x)
				s.Services[i] = &x
			}
		}
	}
}
func IngressServiceToStructs(s *IngressService, t *structs.IngressService) {
	if s == nil {
		return
	}
	t.Name = s.Name
	t.Hosts = s.Hosts
	if s.TLS != nil {
		var x structs.GatewayServiceTLSConfig
		GatewayServiceTLSConfigToStructs(s.TLS, &x)
		t.TLS = &x
	}
	if s.RequestHeaders != nil {
		var x structs.HTTPHeaderModifiers
		HTTPHeaderModifiersToStructs(s.RequestHeaders, &x)
		t.RequestHeaders = &x
	}
	if s.ResponseHeaders != nil {
		var x structs.HTTPHeaderModifiers
		HTTPHeaderModifiersToStructs(s.ResponseHeaders, &x)
		t.ResponseHeaders = &x
	}
	t.MaxConnections = s.MaxConnections
	t.MaxPendingRequests = s.MaxPendingRequests
	t.MaxConcurrentRequests = s.MaxConcurrentRequests
	if s.PassiveHealthCheck != nil {
		var x structs.PassiveHealthCheck
		PassiveHealthCheckToStructs(s.PassiveHealthCheck, &x)
		t.PassiveHealthCheck = &x
	}
	t.Meta = s.Meta
	t.EnterpriseMeta = enterpriseMetaToStructs(s.EnterpriseMeta)
}
func IngressServiceFromStructs(t *structs.IngressService, s *IngressService) {
	if s == nil {
		return
	}
	s.Name = t.Name
	s.Hosts = t.Hosts
	if t.TLS != nil {
		var x GatewayServiceTLSConfig
		GatewayServiceTLSConfigFromStructs(t.TLS, &x)
		s.TLS = &x
	}
	if t.RequestHeaders != nil {
		var x HTTPHeaderModifiers
		HTTPHeaderModifiersFromStructs(t.RequestHeaders, &x)
		s.RequestHeaders = &x
	}
	if t.ResponseHeaders != nil {
		var x HTTPHeaderModifiers
		HTTPHeaderModifiersFromStructs(t.ResponseHeaders, &x)
		s.ResponseHeaders = &x
	}
	s.MaxConnections = t.MaxConnections
	s.MaxPendingRequests = t.MaxPendingRequests
	s.MaxConcurrentRequests = t.MaxConcurrentRequests
	if t.PassiveHealthCheck != nil {
		var x PassiveHealthCheck
		PassiveHealthCheckFromStructs(t.PassiveHealthCheck, &x)
		s.PassiveHealthCheck = &x
	}
	s.Meta = t.Meta
	s.EnterpriseMeta = enterpriseMetaFromStructs(t.EnterpriseMeta)
}
func IngressServiceConfigToStructs(s *IngressServiceConfig, t *structs.IngressServiceConfig) {
	if s == nil {
		return
	}
	t.MaxConnections = s.MaxConnections
	t.MaxPendingRequests = s.MaxPendingRequests
	t.MaxConcurrentRequests = s.MaxConcurrentRequests
	if s.PassiveHealthCheck != nil {
		var x structs.PassiveHealthCheck
		PassiveHealthCheckToStructs(s.PassiveHealthCheck, &x)
		t.PassiveHealthCheck = &x
	}
}
func IngressServiceConfigFromStructs(t *structs.IngressServiceConfig, s *IngressServiceConfig) {
	if s == nil {
		return
	}
	s.MaxConnections = t.MaxConnections
	s.MaxPendingRequests = t.MaxPendingRequests
	s.MaxConcurrentRequests = t.MaxConcurrentRequests
	if t.PassiveHealthCheck != nil {
		var x PassiveHealthCheck
		PassiveHealthCheckFromStructs(t.PassiveHealthCheck, &x)
		s.PassiveHealthCheck = &x
	}
}
func IntentionHTTPHeaderPermissionToStructs(s *IntentionHTTPHeaderPermission, t *structs.IntentionHTTPHeaderPermission) {
	if s == nil {
		return
	}
	t.Name = s.Name
	t.Present = s.Present
	t.Exact = s.Exact
	t.Prefix = s.Prefix
	t.Suffix = s.Suffix
	t.Regex = s.Regex
	t.Invert = s.Invert
}
func IntentionHTTPHeaderPermissionFromStructs(t *structs.IntentionHTTPHeaderPermission, s *IntentionHTTPHeaderPermission) {
	if s == nil {
		return
	}
	s.Name = t.Name
	s.Present = t.Present
	s.Exact = t.Exact
	s.Prefix = t.Prefix
	s.Suffix = t.Suffix
	s.Regex = t.Regex
	s.Invert = t.Invert
}
func IntentionHTTPPermissionToStructs(s *IntentionHTTPPermission, t *structs.IntentionHTTPPermission) {
	if s == nil {
		return
	}
	t.PathExact = s.PathExact
	t.PathPrefix = s.PathPrefix
	t.PathRegex = s.PathRegex
	{
		t.Header = make([]structs.IntentionHTTPHeaderPermission, len(s.Header))
		for i := range s.Header {
			if s.Header[i] != nil {
				IntentionHTTPHeaderPermissionToStructs(s.Header[i], &t.Header[i])
			}
		}
	}
	t.Methods = s.Methods
}
func IntentionHTTPPermissionFromStructs(t *structs.IntentionHTTPPermission, s *IntentionHTTPPermission) {
	if s == nil {
		return
	}
	s.PathExact = t.PathExact
	s.PathPrefix = t.PathPrefix
	s.PathRegex = t.PathRegex
	{
		s.Header = make([]*IntentionHTTPHeaderPermission, len(t.Header))
		for i := range t.Header {
			{
				var x IntentionHTTPHeaderPermission
				IntentionHTTPHeaderPermissionFromStructs(&t.Header[i], &x)
				s.Header[i] = &x
			}
		}
	}
	s.Methods = t.Methods
}
func IntentionPermissionToStructs(s *IntentionPermission, t *structs.IntentionPermission) {
	if s == nil {
		return
	}
	t.Action = intentionActionToStructs(s.Action)
	if s.HTTP != nil {
		var x structs.IntentionHTTPPermission
		IntentionHTTPPermissionToStructs(s.HTTP, &x)
		t.HTTP = &x
	}
}
func IntentionPermissionFromStructs(t *structs.IntentionPermission, s *IntentionPermission) {
	if s == nil {
		return
	}
	s.Action = intentionActionFromStructs(t.Action)
	if t.HTTP != nil {
		var x IntentionHTTPPermission
		IntentionHTTPPermissionFromStructs(t.HTTP, &x)
		s.HTTP = &x
	}
}
func LeastRequestConfigToStructs(s *LeastRequestConfig, t *structs.LeastRequestConfig) {
	if s == nil {
		return
	}
	t.ChoiceCount = s.ChoiceCount
}
func LeastRequestConfigFromStructs(t *structs.LeastRequestConfig, s *LeastRequestConfig) {
	if s == nil {
		return
	}
	s.ChoiceCount = t.ChoiceCount
}
func LoadBalancerToStructs(s *LoadBalancer, t *structs.LoadBalancer) {
	if s == nil {
		return
	}
	t.Policy = s.Policy
	if s.RingHashConfig != nil {
		var x structs.RingHashConfig
		RingHashConfigToStructs(s.RingHashConfig, &x)
		t.RingHashConfig = &x
	}
	if s.LeastRequestConfig != nil {
		var x structs.LeastRequestConfig
		LeastRequestConfigToStructs(s.LeastRequestConfig, &x)
		t.LeastRequestConfig = &x
	}
	{
		t.HashPolicies = make([]structs.HashPolicy, len(s.HashPolicies))
		for i := range s.HashPolicies {
			if s.HashPolicies[i] != nil {
				HashPolicyToStructs(s.HashPolicies[i], &t.HashPolicies[i])
			}
		}
	}
}
func LoadBalancerFromStructs(t *structs.LoadBalancer, s *LoadBalancer) {
	if s == nil {
		return
	}
	s.Policy = t.Policy
	if t.RingHashConfig != nil {
		var x RingHashConfig
		RingHashConfigFromStructs(t.RingHashConfig, &x)
		s.RingHashConfig = &x
	}
	if t.LeastRequestConfig != nil {
		var x LeastRequestConfig
		LeastRequestConfigFromStructs(t.LeastRequestConfig, &x)
		s.LeastRequestConfig = &x
	}
	{
		s.HashPolicies = make([]*HashPolicy, len(t.HashPolicies))
		for i := range t.HashPolicies {
			{
				var x HashPolicy
				HashPolicyFromStructs(&t.HashPolicies[i], &x)
				s.HashPolicies[i] = &x
			}
		}
	}
}
func MeshConfigToStructs(s *MeshConfig, t *structs.MeshConfigEntry) {
	if s == nil {
		return
	}
	if s.TransparentProxy != nil {
		TransparentProxyMeshConfigToStructs(s.TransparentProxy, &t.TransparentProxy)
	}
	if s.TLS != nil {
		var x structs.MeshTLSConfig
		MeshTLSConfigToStructs(s.TLS, &x)
		t.TLS = &x
	}
	if s.HTTP != nil {
		var x structs.MeshHTTPConfig
		MeshHTTPConfigToStructs(s.HTTP, &x)
		t.HTTP = &x
	}
	if s.Peering != nil {
		var x structs.PeeringMeshConfig
		PeeringMeshConfigToStructs(s.Peering, &x)
		t.Peering = &x
	}
	t.Meta = s.Meta
}
func MeshConfigFromStructs(t *structs.MeshConfigEntry, s *MeshConfig) {
	if s == nil {
		return
	}
	{
		var x TransparentProxyMeshConfig
		TransparentProxyMeshConfigFromStructs(&t.TransparentProxy, &x)
		s.TransparentProxy = &x
	}
	if t.TLS != nil {
		var x MeshTLSConfig
		MeshTLSConfigFromStructs(t.TLS, &x)
		s.TLS = &x
	}
	if t.HTTP != nil {
		var x MeshHTTPConfig
		MeshHTTPConfigFromStructs(t.HTTP, &x)
		s.HTTP = &x
	}
	if t.Peering != nil {
		var x PeeringMeshConfig
		PeeringMeshConfigFromStructs(t.Peering, &x)
		s.Peering = &x
	}
	s.Meta = t.Meta
}
func MeshDirectionalTLSConfigToStructs(s *MeshDirectionalTLSConfig, t *structs.MeshDirectionalTLSConfig) {
	if s == nil {
		return
	}
	t.TLSMinVersion = tlsVersionToStructs(s.TLSMinVersion)
	t.TLSMaxVersion = tlsVersionToStructs(s.TLSMaxVersion)
	t.CipherSuites = cipherSuitesToStructs(s.CipherSuites)
}
func MeshDirectionalTLSConfigFromStructs(t *structs.MeshDirectionalTLSConfig, s *MeshDirectionalTLSConfig) {
	if s == nil {
		return
	}
	s.TLSMinVersion = tlsVersionFromStructs(t.TLSMinVersion)
	s.TLSMaxVersion = tlsVersionFromStructs(t.TLSMaxVersion)
	s.CipherSuites = cipherSuitesFromStructs(t.CipherSuites)
}
func MeshGatewayConfigToStructs(s *MeshGatewayConfig, t *structs.MeshGatewayConfig) {
	if s == nil {
		return
	}
	t.Mode = meshGatewayModeToStructs(s.Mode)
}
func MeshGatewayConfigFromStructs(t *structs.MeshGatewayConfig, s *MeshGatewayConfig) {
	if s == nil {
		return
	}
	s.Mode = meshGatewayModeFromStructs(t.Mode)
}
func MeshHTTPConfigToStructs(s *MeshHTTPConfig, t *structs.MeshHTTPConfig) {
	if s == nil {
		return
	}
	t.SanitizeXForwardedClientCert = s.SanitizeXForwardedClientCert
}
func MeshHTTPConfigFromStructs(t *structs.MeshHTTPConfig, s *MeshHTTPConfig) {
	if s == nil {
		return
	}
	s.SanitizeXForwardedClientCert = t.SanitizeXForwardedClientCert
}
func MeshTLSConfigToStructs(s *MeshTLSConfig, t *structs.MeshTLSConfig) {
	if s == nil {
		return
	}
	if s.Incoming != nil {
		var x structs.MeshDirectionalTLSConfig
		MeshDirectionalTLSConfigToStructs(s.Incoming, &x)
		t.Incoming = &x
	}
	if s.Outgoing != nil {
		var x structs.MeshDirectionalTLSConfig
		MeshDirectionalTLSConfigToStructs(s.Outgoing, &x)
		t.Outgoing = &x
	}
}
func MeshTLSConfigFromStructs(t *structs.MeshTLSConfig, s *MeshTLSConfig) {
	if s == nil {
		return
	}
	if t.Incoming != nil {
		var x MeshDirectionalTLSConfig
		MeshDirectionalTLSConfigFromStructs(t.Incoming, &x)
		s.Incoming = &x
	}
	if t.Outgoing != nil {
		var x MeshDirectionalTLSConfig
		MeshDirectionalTLSConfigFromStructs(t.Outgoing, &x)
		s.Outgoing = &x
	}
}
func PassiveHealthCheckToStructs(s *PassiveHealthCheck, t *structs.PassiveHealthCheck) {
	if s == nil {
		return
	}
	t.Interval = structs.DurationFromProto(s.Interval)
	t.MaxFailures = s.MaxFailures
	t.EnforcingConsecutive5xx = pointerToUint32FromUint32(s.EnforcingConsecutive5Xx)
}
func PassiveHealthCheckFromStructs(t *structs.PassiveHealthCheck, s *PassiveHealthCheck) {
	if s == nil {
		return
	}
	s.Interval = structs.DurationToProto(t.Interval)
	s.MaxFailures = t.MaxFailures
	s.EnforcingConsecutive5Xx = uint32FromPointerToUint32(t.EnforcingConsecutive5xx)
}
func PeeringMeshConfigToStructs(s *PeeringMeshConfig, t *structs.PeeringMeshConfig) {
	if s == nil {
		return
	}
	t.PeerThroughMeshGateways = s.PeerThroughMeshGateways
}
func PeeringMeshConfigFromStructs(t *structs.PeeringMeshConfig, s *PeeringMeshConfig) {
	if s == nil {
		return
	}
	s.PeerThroughMeshGateways = t.PeerThroughMeshGateways
}
func RingHashConfigToStructs(s *RingHashConfig, t *structs.RingHashConfig) {
	if s == nil {
		return
	}
	t.MinimumRingSize = s.MinimumRingSize
	t.MaximumRingSize = s.MaximumRingSize
}
func RingHashConfigFromStructs(t *structs.RingHashConfig, s *RingHashConfig) {
	if s == nil {
		return
	}
	s.MinimumRingSize = t.MinimumRingSize
	s.MaximumRingSize = t.MaximumRingSize
}
func ServiceDefaultsToStructs(s *ServiceDefaults, t *structs.ServiceConfigEntry) {
	if s == nil {
		return
	}
	t.Protocol = s.Protocol
	t.Mode = proxyModeToStructs(s.Mode)
	if s.TransparentProxy != nil {
		TransparentProxyConfigToStructs(s.TransparentProxy, &t.TransparentProxy)
	}
	if s.MeshGateway != nil {
		MeshGatewayConfigToStructs(s.MeshGateway, &t.MeshGateway)
	}
	if s.Expose != nil {
		ExposeConfigToStructs(s.Expose, &t.Expose)
	}
	t.ExternalSNI = s.ExternalSNI
	if s.UpstreamConfig != nil {
		var x structs.UpstreamConfiguration
		UpstreamConfigurationToStructs(s.UpstreamConfig, &x)
		t.UpstreamConfig = &x
	}
	if s.Destination != nil {
		var x structs.DestinationConfig
		DestinationConfigToStructs(s.Destination, &x)
		t.Destination = &x
	}
	t.MaxInboundConnections = int(s.MaxInboundConnections)
	t.LocalConnectTimeoutMs = int(s.LocalConnectTimeoutMs)
	t.LocalRequestTimeoutMs = int(s.LocalRequestTimeoutMs)
	t.BalanceInboundConnections = s.BalanceInboundConnections
	{
		t.EnvoyExtensions = make([]structs.EnvoyExtension, len(s.EnvoyExtensions))
		for i := range s.EnvoyExtensions {
			if s.EnvoyExtensions[i] != nil {
				EnvoyExtensionToStructs(s.EnvoyExtensions[i], &t.EnvoyExtensions[i])
			}
		}
	}
	t.Meta = s.Meta
}
func ServiceDefaultsFromStructs(t *structs.ServiceConfigEntry, s *ServiceDefaults) {
	if s == nil {
		return
	}
	s.Protocol = t.Protocol
	s.Mode = proxyModeFromStructs(t.Mode)
	{
		var x TransparentProxyConfig
		TransparentProxyConfigFromStructs(&t.TransparentProxy, &x)
		s.TransparentProxy = &x
	}
	{
		var x MeshGatewayConfig
		MeshGatewayConfigFromStructs(&t.MeshGateway, &x)
		s.MeshGateway = &x
	}
	{
		var x ExposeConfig
		ExposeConfigFromStructs(&t.Expose, &x)
		s.Expose = &x
	}
	s.ExternalSNI = t.ExternalSNI
	if t.UpstreamConfig != nil {
		var x UpstreamConfiguration
		UpstreamConfigurationFromStructs(t.UpstreamConfig, &x)
		s.UpstreamConfig = &x
	}
	if t.Destination != nil {
		var x DestinationConfig
		DestinationConfigFromStructs(t.Destination, &x)
		s.Destination = &x
	}
	s.MaxInboundConnections = int32(t.MaxInboundConnections)
	s.LocalConnectTimeoutMs = int32(t.LocalConnectTimeoutMs)
	s.LocalRequestTimeoutMs = int32(t.LocalRequestTimeoutMs)
	s.BalanceInboundConnections = t.BalanceInboundConnections
	{
		s.EnvoyExtensions = make([]*EnvoyExtension, len(t.EnvoyExtensions))
		for i := range t.EnvoyExtensions {
			{
				var x EnvoyExtension
				EnvoyExtensionFromStructs(&t.EnvoyExtensions[i], &x)
				s.EnvoyExtensions[i] = &x
			}
		}
	}
	s.Meta = t.Meta
}
func ServiceIntentionsToStructs(s *ServiceIntentions, t *structs.ServiceIntentionsConfigEntry) {
	if s == nil {
		return
	}
	{
		t.Sources = make([]*structs.SourceIntention, len(s.Sources))
		for i := range s.Sources {
			if s.Sources[i] != nil {
				var x structs.SourceIntention
				SourceIntentionToStructs(s.Sources[i], &x)
				t.Sources[i] = &x
			}
		}
	}
	t.Meta = s.Meta
}
func ServiceIntentionsFromStructs(t *structs.ServiceIntentionsConfigEntry, s *ServiceIntentions) {
	if s == nil {
		return
	}
	{
		s.Sources = make([]*SourceIntention, len(t.Sources))
		for i := range t.Sources {
			if t.Sources[i] != nil {
				var x SourceIntention
				SourceIntentionFromStructs(t.Sources[i], &x)
				s.Sources[i] = &x
			}
		}
	}
	s.Meta = t.Meta
}
func ServiceResolverToStructs(s *ServiceResolver, t *structs.ServiceResolverConfigEntry) {
	if s == nil {
		return
	}
	t.DefaultSubset = s.DefaultSubset
	{
		t.Subsets = make(map[string]structs.ServiceResolverSubset, len(s.Subsets))
		for k, v := range s.Subsets {
			var y structs.ServiceResolverSubset
			if v != nil {
				ServiceResolverSubsetToStructs(v, &y)
			}
			t.Subsets[k] = y
		}
	}
	if s.Redirect != nil {
		var x structs.ServiceResolverRedirect
		ServiceResolverRedirectToStructs(s.Redirect, &x)
		t.Redirect = &x
	}
	{
		t.Failover = make(map[string]structs.ServiceResolverFailover, len(s.Failover))
		for k, v := range s.Failover {
			var y structs.ServiceResolverFailover
			if v != nil {
				ServiceResolverFailoverToStructs(v, &y)
			}
			t.Failover[k] = y
		}
	}
	t.ConnectTimeout = structs.DurationFromProto(s.ConnectTimeout)
	if s.LoadBalancer != nil {
		var x structs.LoadBalancer
		LoadBalancerToStructs(s.LoadBalancer, &x)
		t.LoadBalancer = &x
	}
	t.Meta = s.Meta
}
func ServiceResolverFromStructs(t *structs.ServiceResolverConfigEntry, s *ServiceResolver) {
	if s == nil {
		return
	}
	s.DefaultSubset = t.DefaultSubset
	{
		s.Subsets = make(map[string]*ServiceResolverSubset, len(t.Subsets))
		for k, v := range t.Subsets {
			var y *ServiceResolverSubset
			{
				var x ServiceResolverSubset
				ServiceResolverSubsetFromStructs(&v, &x)
				y = &x
			}
			s.Subsets[k] = y
		}
	}
	if t.Redirect != nil {
		var x ServiceResolverRedirect
		ServiceResolverRedirectFromStructs(t.Redirect, &x)
		s.Redirect = &x
	}
	{
		s.Failover = make(map[string]*ServiceResolverFailover, len(t.Failover))
		for k, v := range t.Failover {
			var y *ServiceResolverFailover
			{
				var x ServiceResolverFailover
				ServiceResolverFailoverFromStructs(&v, &x)
				y = &x
			}
			s.Failover[k] = y
		}
	}
	s.ConnectTimeout = structs.DurationToProto(t.ConnectTimeout)
	if t.LoadBalancer != nil {
		var x LoadBalancer
		LoadBalancerFromStructs(t.LoadBalancer, &x)
		s.LoadBalancer = &x
	}
	s.Meta = t.Meta
}
func ServiceResolverFailoverToStructs(s *ServiceResolverFailover, t *structs.ServiceResolverFailover) {
	if s == nil {
		return
	}
	t.Service = s.Service
	t.ServiceSubset = s.ServiceSubset
	t.Namespace = s.Namespace
	t.Datacenters = s.Datacenters
	{
		t.Targets = make([]structs.ServiceResolverFailoverTarget, len(s.Targets))
		for i := range s.Targets {
			if s.Targets[i] != nil {
				ServiceResolverFailoverTargetToStructs(s.Targets[i], &t.Targets[i])
			}
		}
	}
}
func ServiceResolverFailoverFromStructs(t *structs.ServiceResolverFailover, s *ServiceResolverFailover) {
	if s == nil {
		return
	}
	s.Service = t.Service
	s.ServiceSubset = t.ServiceSubset
	s.Namespace = t.Namespace
	s.Datacenters = t.Datacenters
	{
		s.Targets = make([]*ServiceResolverFailoverTarget, len(t.Targets))
		for i := range t.Targets {
			{
				var x ServiceResolverFailoverTarget
				ServiceResolverFailoverTargetFromStructs(&t.Targets[i], &x)
				s.Targets[i] = &x
			}
		}
	}
}
func ServiceResolverFailoverTargetToStructs(s *ServiceResolverFailoverTarget, t *structs.ServiceResolverFailoverTarget) {
	if s == nil {
		return
	}
	t.Service = s.Service
	t.ServiceSubset = s.ServiceSubset
	t.Partition = s.Partition
	t.Namespace = s.Namespace
	t.Datacenter = s.Datacenter
	t.Peer = s.Peer
}
func ServiceResolverFailoverTargetFromStructs(t *structs.ServiceResolverFailoverTarget, s *ServiceResolverFailoverTarget) {
	if s == nil {
		return
	}
	s.Service = t.Service
	s.ServiceSubset = t.ServiceSubset
	s.Partition = t.Partition
	s.Namespace = t.Namespace
	s.Datacenter = t.Datacenter
	s.Peer = t.Peer
}
func ServiceResolverRedirectToStructs(s *ServiceResolverRedirect, t *structs.ServiceResolverRedirect) {
	if s == nil {
		return
	}
	t.Service = s.Service
	t.ServiceSubset = s.ServiceSubset
	t.Namespace = s.Namespace
	t.Partition = s.Partition
	t.Datacenter = s.Datacenter
	t.Peer = s.Peer
}
func ServiceResolverRedirectFromStructs(t *structs.ServiceResolverRedirect, s *ServiceResolverRedirect) {
	if s == nil {
		return
	}
	s.Service = t.Service
	s.ServiceSubset = t.ServiceSubset
	s.Namespace = t.Namespace
	s.Partition = t.Partition
	s.Datacenter = t.Datacenter
	s.Peer = t.Peer
}
func ServiceResolverSubsetToStructs(s *ServiceResolverSubset, t *structs.ServiceResolverSubset) {
	if s == nil {
		return
	}
	t.Filter = s.Filter
	t.OnlyPassing = s.OnlyPassing
}
func ServiceResolverSubsetFromStructs(t *structs.ServiceResolverSubset, s *ServiceResolverSubset) {
	if s == nil {
		return
	}
	s.Filter = t.Filter
	s.OnlyPassing = t.OnlyPassing
}
func SourceIntentionToStructs(s *SourceIntention, t *structs.SourceIntention) {
	if s == nil {
		return
	}
	t.Name = s.Name
	t.Action = intentionActionToStructs(s.Action)
	{
		t.Permissions = make([]*structs.IntentionPermission, len(s.Permissions))
		for i := range s.Permissions {
			if s.Permissions[i] != nil {
				var x structs.IntentionPermission
				IntentionPermissionToStructs(s.Permissions[i], &x)
				t.Permissions[i] = &x
			}
		}
	}
	t.Precedence = int(s.Precedence)
	t.LegacyID = s.LegacyID
	t.Type = intentionSourceTypeToStructs(s.Type)
	t.Description = s.Description
	t.LegacyMeta = s.LegacyMeta
	t.LegacyCreateTime = timeToStructs(s.LegacyCreateTime)
	t.LegacyUpdateTime = timeToStructs(s.LegacyUpdateTime)
	t.EnterpriseMeta = enterpriseMetaToStructs(s.EnterpriseMeta)
	t.Peer = s.Peer
}
func SourceIntentionFromStructs(t *structs.SourceIntention, s *SourceIntention) {
	if s == nil {
		return
	}
	s.Name = t.Name
	s.Action = intentionActionFromStructs(t.Action)
	{
		s.Permissions = make([]*IntentionPermission, len(t.Permissions))
		for i := range t.Permissions {
			if t.Permissions[i] != nil {
				var x IntentionPermission
				IntentionPermissionFromStructs(t.Permissions[i], &x)
				s.Permissions[i] = &x
			}
		}
	}
	s.Precedence = int32(t.Precedence)
	s.LegacyID = t.LegacyID
	s.Type = intentionSourceTypeFromStructs(t.Type)
	s.Description = t.Description
	s.LegacyMeta = t.LegacyMeta
	s.LegacyCreateTime = timeFromStructs(t.LegacyCreateTime)
	s.LegacyUpdateTime = timeFromStructs(t.LegacyUpdateTime)
	s.EnterpriseMeta = enterpriseMetaFromStructs(t.EnterpriseMeta)
	s.Peer = t.Peer
}
func TransparentProxyConfigToStructs(s *TransparentProxyConfig, t *structs.TransparentProxyConfig) {
	if s == nil {
		return
	}
	t.OutboundListenerPort = int(s.OutboundListenerPort)
	t.DialedDirectly = s.DialedDirectly
}
func TransparentProxyConfigFromStructs(t *structs.TransparentProxyConfig, s *TransparentProxyConfig) {
	if s == nil {
		return
	}
	s.OutboundListenerPort = int32(t.OutboundListenerPort)
	s.DialedDirectly = t.DialedDirectly
}
func TransparentProxyMeshConfigToStructs(s *TransparentProxyMeshConfig, t *structs.TransparentProxyMeshConfig) {
	if s == nil {
		return
	}
	t.MeshDestinationsOnly = s.MeshDestinationsOnly
}
func TransparentProxyMeshConfigFromStructs(t *structs.TransparentProxyMeshConfig, s *TransparentProxyMeshConfig) {
	if s == nil {
		return
	}
	s.MeshDestinationsOnly = t.MeshDestinationsOnly
}
func UpstreamConfigToStructs(s *UpstreamConfig, t *structs.UpstreamConfig) {
	if s == nil {
		return
	}
	t.Name = s.Name
	t.EnterpriseMeta = enterpriseMetaToStructs(s.EnterpriseMeta)
	t.EnvoyListenerJSON = s.EnvoyListenerJSON
	t.EnvoyClusterJSON = s.EnvoyClusterJSON
	t.Protocol = s.Protocol
	t.ConnectTimeoutMs = int(s.ConnectTimeoutMs)
	if s.Limits != nil {
		var x structs.UpstreamLimits
		UpstreamLimitsToStructs(s.Limits, &x)
		t.Limits = &x
	}
	if s.PassiveHealthCheck != nil {
		var x structs.PassiveHealthCheck
		PassiveHealthCheckToStructs(s.PassiveHealthCheck, &x)
		t.PassiveHealthCheck = &x
	}
	if s.MeshGateway != nil {
		MeshGatewayConfigToStructs(s.MeshGateway, &t.MeshGateway)
	}
	t.BalanceOutboundConnections = s.BalanceOutboundConnections
}
func UpstreamConfigFromStructs(t *structs.UpstreamConfig, s *UpstreamConfig) {
	if s == nil {
		return
	}
	s.Name = t.Name
	s.EnterpriseMeta = enterpriseMetaFromStructs(t.EnterpriseMeta)
	s.EnvoyListenerJSON = t.EnvoyListenerJSON
	s.EnvoyClusterJSON = t.EnvoyClusterJSON
	s.Protocol = t.Protocol
	s.ConnectTimeoutMs = int32(t.ConnectTimeoutMs)
	if t.Limits != nil {
		var x UpstreamLimits
		UpstreamLimitsFromStructs(t.Limits, &x)
		s.Limits = &x
	}
	if t.PassiveHealthCheck != nil {
		var x PassiveHealthCheck
		PassiveHealthCheckFromStructs(t.PassiveHealthCheck, &x)
		s.PassiveHealthCheck = &x
	}
	{
		var x MeshGatewayConfig
		MeshGatewayConfigFromStructs(&t.MeshGateway, &x)
		s.MeshGateway = &x
	}
	s.BalanceOutboundConnections = t.BalanceOutboundConnections
}
func UpstreamConfigurationToStructs(s *UpstreamConfiguration, t *structs.UpstreamConfiguration) {
	if s == nil {
		return
	}
	{
		t.Overrides = make([]*structs.UpstreamConfig, len(s.Overrides))
		for i := range s.Overrides {
			if s.Overrides[i] != nil {
				var x structs.UpstreamConfig
				UpstreamConfigToStructs(s.Overrides[i], &x)
				t.Overrides[i] = &x
			}
		}
	}
	if s.Defaults != nil {
		var x structs.UpstreamConfig
		UpstreamConfigToStructs(s.Defaults, &x)
		t.Defaults = &x
	}
}
func UpstreamConfigurationFromStructs(t *structs.UpstreamConfiguration, s *UpstreamConfiguration) {
	if s == nil {
		return
	}
	{
		s.Overrides = make([]*UpstreamConfig, len(t.Overrides))
		for i := range t.Overrides {
			if t.Overrides[i] != nil {
				var x UpstreamConfig
				UpstreamConfigFromStructs(t.Overrides[i], &x)
				s.Overrides[i] = &x
			}
		}
	}
	if t.Defaults != nil {
		var x UpstreamConfig
		UpstreamConfigFromStructs(t.Defaults, &x)
		s.Defaults = &x
	}
}
func UpstreamLimitsToStructs(s *UpstreamLimits, t *structs.UpstreamLimits) {
	if s == nil {
		return
	}
	t.MaxConnections = pointerToIntFromInt32(s.MaxConnections)
	t.MaxPendingRequests = pointerToIntFromInt32(s.MaxPendingRequests)
	t.MaxConcurrentRequests = pointerToIntFromInt32(s.MaxConcurrentRequests)
}
func UpstreamLimitsFromStructs(t *structs.UpstreamLimits, s *UpstreamLimits) {
	if s == nil {
		return
	}
	s.MaxConnections = int32FromPointerToInt(t.MaxConnections)
	s.MaxPendingRequests = int32FromPointerToInt(t.MaxPendingRequests)
	s.MaxConcurrentRequests = int32FromPointerToInt(t.MaxConcurrentRequests)
}
