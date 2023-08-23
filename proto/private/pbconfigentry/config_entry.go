// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pbconfigentry

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/types"
)

// Function variables to support proto generation
// This allows for using functions in the local package without having to generate imports
var EnvoyExtensionsToStructs = pbcommon.EnvoyExtensionsToStructs
var EnvoyExtensionsFromStructs = pbcommon.EnvoyExtensionsFromStructs

func ConfigEntryToStructs(s *ConfigEntry) structs.ConfigEntry {
	switch s.Kind {
	case Kind_KindMeshConfig:
		var target structs.MeshConfigEntry
		MeshConfigToStructs(s.GetMeshConfig(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindServiceResolver:
		var target structs.ServiceResolverConfigEntry
		target.Name = s.Name

		ServiceResolverToStructs(s.GetServiceResolver(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindIngressGateway:
		var target structs.IngressGatewayConfigEntry
		target.Name = s.Name

		IngressGatewayToStructs(s.GetIngressGateway(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindServiceIntentions:
		var target structs.ServiceIntentionsConfigEntry
		target.Name = s.Name

		ServiceIntentionsToStructs(s.GetServiceIntentions(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindAPIGateway:
		var target structs.APIGatewayConfigEntry
		target.Name = s.Name

		APIGatewayToStructs(s.GetAPIGateway(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindBoundAPIGateway:
		var target structs.BoundAPIGatewayConfigEntry
		target.Name = s.Name

		BoundAPIGatewayToStructs(s.GetBoundAPIGateway(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindTCPRoute:
		var target structs.TCPRouteConfigEntry
		target.Name = s.Name

		TCPRouteToStructs(s.GetTCPRoute(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindHTTPRoute:
		var target structs.HTTPRouteConfigEntry
		target.Name = s.Name

		HTTPRouteToStructs(s.GetHTTPRoute(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindInlineCertificate:
		var target structs.InlineCertificateConfigEntry
		target.Name = s.Name

		InlineCertificateToStructs(s.GetInlineCertificate(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindServiceDefaults:
		var target structs.ServiceConfigEntry
		target.Name = s.Name

		ServiceDefaultsToStructs(s.GetServiceDefaults(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindSamenessGroup:
		var target structs.SamenessGroupConfigEntry
		target.Name = s.Name

		SamenessGroupToStructs(s.GetSamenessGroup(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	case Kind_KindJWTProvider:
		var target structs.JWTProviderConfigEntry
		target.Name = s.Name

		JWTProviderToStructs(s.GetJWTProvider(), &target)
		pbcommon.RaftIndexToStructs(s.RaftIndex, &target.RaftIndex)
		pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta, &target.EnterpriseMeta)
		return &target
	default:
		panic(fmt.Sprintf("unable to convert ConfigEntry of kind %s to structs", s.Kind))
	}
}

func ConfigEntryFromStructs(s structs.ConfigEntry) *ConfigEntry {
	configEntry := &ConfigEntry{
		Name:           s.GetName(),
		EnterpriseMeta: pbcommon.NewEnterpriseMetaFromStructs(*s.GetEnterpriseMeta()),
	}

	var raftIndex pbcommon.RaftIndex
	pbcommon.RaftIndexFromStructs(s.GetRaftIndex(), &raftIndex)
	configEntry.RaftIndex = &raftIndex

	switch v := s.(type) {
	case *structs.MeshConfigEntry:
		var meshConfig MeshConfig
		MeshConfigFromStructs(v, &meshConfig)

		configEntry.Kind = Kind_KindMeshConfig
		configEntry.Entry = &ConfigEntry_MeshConfig{
			MeshConfig: &meshConfig,
		}
	case *structs.ServiceResolverConfigEntry:
		var serviceResolver ServiceResolver
		ServiceResolverFromStructs(v, &serviceResolver)

		configEntry.Kind = Kind_KindServiceResolver
		configEntry.Entry = &ConfigEntry_ServiceResolver{
			ServiceResolver: &serviceResolver,
		}
	case *structs.IngressGatewayConfigEntry:
		var ingressGateway IngressGateway
		IngressGatewayFromStructs(v, &ingressGateway)

		configEntry.Kind = Kind_KindIngressGateway
		configEntry.Entry = &ConfigEntry_IngressGateway{
			IngressGateway: &ingressGateway,
		}
	case *structs.ServiceIntentionsConfigEntry:
		var serviceIntentions ServiceIntentions
		ServiceIntentionsFromStructs(v, &serviceIntentions)

		configEntry.Kind = Kind_KindServiceIntentions
		configEntry.Entry = &ConfigEntry_ServiceIntentions{
			ServiceIntentions: &serviceIntentions,
		}
	case *structs.ServiceConfigEntry:
		var serviceDefaults ServiceDefaults
		ServiceDefaultsFromStructs(v, &serviceDefaults)

		configEntry.Kind = Kind_KindServiceDefaults
		configEntry.Entry = &ConfigEntry_ServiceDefaults{
			ServiceDefaults: &serviceDefaults,
		}
	case *structs.APIGatewayConfigEntry:
		var apiGateway APIGateway
		APIGatewayFromStructs(v, &apiGateway)

		configEntry.Kind = Kind_KindAPIGateway
		configEntry.Entry = &ConfigEntry_APIGateway{
			APIGateway: &apiGateway,
		}
	case *structs.BoundAPIGatewayConfigEntry:
		var apiGateway BoundAPIGateway
		BoundAPIGatewayFromStructs(v, &apiGateway)

		configEntry.Kind = Kind_KindBoundAPIGateway
		configEntry.Entry = &ConfigEntry_BoundAPIGateway{
			BoundAPIGateway: &apiGateway,
		}
	case *structs.TCPRouteConfigEntry:
		var route TCPRoute
		TCPRouteFromStructs(v, &route)

		configEntry.Kind = Kind_KindTCPRoute
		configEntry.Entry = &ConfigEntry_TCPRoute{
			TCPRoute: &route,
		}
	case *structs.HTTPRouteConfigEntry:
		var route HTTPRoute
		HTTPRouteFromStructs(v, &route)

		configEntry.Kind = Kind_KindHTTPRoute
		configEntry.Entry = &ConfigEntry_HTTPRoute{
			HTTPRoute: &route,
		}
	case *structs.InlineCertificateConfigEntry:
		var cert InlineCertificate
		InlineCertificateFromStructs(v, &cert)

		configEntry.Kind = Kind_KindInlineCertificate
		configEntry.Entry = &ConfigEntry_InlineCertificate{
			InlineCertificate: &cert,
		}
	case *structs.SamenessGroupConfigEntry:
		var sg SamenessGroup
		SamenessGroupFromStructs(v, &sg)
		configEntry.Kind = Kind_KindSamenessGroup
		configEntry.Entry = &ConfigEntry_SamenessGroup{
			SamenessGroup: &sg,
		}
	case *structs.JWTProviderConfigEntry:
		var jwtProvider JWTProvider
		JWTProviderFromStructs(v, &jwtProvider)

		configEntry.Kind = Kind_KindJWTProvider
		configEntry.Entry = &ConfigEntry_JWTProvider{
			JWTProvider: &jwtProvider,
		}
	default:
		panic(fmt.Sprintf("unable to convert %T to proto", s))
	}

	return configEntry
}

func tlsVersionToStructs(s string) types.TLSVersion {
	return types.TLSVersion(s)
}

func tlsVersionFromStructs(t types.TLSVersion) string {
	return t.String()
}

func cipherSuitesToStructs(cs []string) []types.TLSCipherSuite {
	cipherSuites := make([]types.TLSCipherSuite, len(cs))
	for idx, suite := range cs {
		cipherSuites[idx] = types.TLSCipherSuite(suite)
	}
	return cipherSuites
}

func cipherSuitesFromStructs(cs []types.TLSCipherSuite) []string {
	cipherSuites := make([]string, len(cs))
	for idx, suite := range cs {
		cipherSuites[idx] = suite.String()
	}
	return cipherSuites
}

func enterpriseMetaToStructs(m *pbcommon.EnterpriseMeta) acl.EnterpriseMeta {
	var entMeta acl.EnterpriseMeta
	pbcommon.EnterpriseMetaToStructs(m, &entMeta)
	return entMeta
}

func enterpriseMetaFromStructs(m acl.EnterpriseMeta) *pbcommon.EnterpriseMeta {
	return pbcommon.NewEnterpriseMetaFromStructs(m)
}

func timeFromStructs(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func timeToStructs(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	t := ts.AsTime()
	return &t
}

func intentionActionFromStructs(a structs.IntentionAction) IntentionAction {
	if a == structs.IntentionActionAllow {
		return IntentionAction_Allow
	}
	return IntentionAction_Deny
}

func intentionActionToStructs(a IntentionAction) structs.IntentionAction {
	if a == IntentionAction_Allow {
		return structs.IntentionActionAllow
	}
	return structs.IntentionActionDeny
}

func intentionSourceTypeFromStructs(_ structs.IntentionSourceType) IntentionSourceType {
	return IntentionSourceType_Consul
}

func intentionSourceTypeToStructs(_ IntentionSourceType) structs.IntentionSourceType {
	return structs.IntentionSourceConsul
}

func pointerToIntFromInt32(i32 int32) *int {
	i := int(i32)
	return &i
}

func int32FromPointerToInt(i *int) int32 {
	if i != nil {
		return int32(*i)
	}
	return 0
}

func pointerToUint32FromUint32(ui32 uint32) *uint32 {
	i := ui32
	return &i
}

func uint32FromPointerToUint32(i *uint32) uint32 {
	if i != nil {
		return *i
	}
	return 0
}

func proxyModeFromStructs(a structs.ProxyMode) ProxyMode {
	switch a {
	case structs.ProxyModeDefault:
		return ProxyMode_ProxyModeDefault
	case structs.ProxyModeTransparent:
		return ProxyMode_ProxyModeTransparent
	case structs.ProxyModeDirect:
		return ProxyMode_ProxyModeDirect
	default:
		return ProxyMode_ProxyModeDefault
	}
}

func proxyModeToStructs(a ProxyMode) structs.ProxyMode {
	switch a {
	case ProxyMode_ProxyModeDefault:
		return structs.ProxyModeDefault
	case ProxyMode_ProxyModeTransparent:
		return structs.ProxyModeTransparent
	case ProxyMode_ProxyModeDirect:
		return structs.ProxyModeDirect
	default:
		return structs.ProxyModeDefault
	}
}

func mutualTLSModeFromStructs(a structs.MutualTLSMode) MutualTLSMode {
	switch a {
	case structs.MutualTLSModeDefault:
		return MutualTLSMode_MutualTLSModeDefault
	case structs.MutualTLSModeStrict:
		return MutualTLSMode_MutualTLSModeStrict
	case structs.MutualTLSModePermissive:
		return MutualTLSMode_MutualTLSModePermissive
	default:
		return MutualTLSMode_MutualTLSModeDefault
	}
}

func mutualTLSModeToStructs(a MutualTLSMode) structs.MutualTLSMode {
	switch a {
	case MutualTLSMode_MutualTLSModeDefault:
		return structs.MutualTLSModeDefault
	case MutualTLSMode_MutualTLSModeStrict:
		return structs.MutualTLSModeStrict
	case MutualTLSMode_MutualTLSModePermissive:
		return structs.MutualTLSModePermissive
	default:
		return structs.MutualTLSModeDefault
	}
}

func meshGatewayModeFromStructs(a structs.MeshGatewayMode) MeshGatewayMode {
	switch a {
	case structs.MeshGatewayModeDefault:
		return MeshGatewayMode_MeshGatewayModeDefault
	case structs.MeshGatewayModeNone:
		return MeshGatewayMode_MeshGatewayModeNone
	case structs.MeshGatewayModeLocal:
		return MeshGatewayMode_MeshGatewayModeLocal
	case structs.MeshGatewayModeRemote:
		return MeshGatewayMode_MeshGatewayModeRemote
	default:
		return MeshGatewayMode_MeshGatewayModeDefault
	}
}

func meshGatewayModeToStructs(a MeshGatewayMode) structs.MeshGatewayMode {
	switch a {
	case MeshGatewayMode_MeshGatewayModeDefault:
		return structs.MeshGatewayModeDefault
	case MeshGatewayMode_MeshGatewayModeNone:
		return structs.MeshGatewayModeNone
	case MeshGatewayMode_MeshGatewayModeLocal:
		return structs.MeshGatewayModeLocal
	case MeshGatewayMode_MeshGatewayModeRemote:
		return structs.MeshGatewayModeRemote
	default:
		return structs.MeshGatewayModeDefault
	}
}

func apiGatewayProtocolFromStructs(a structs.APIGatewayListenerProtocol) APIGatewayListenerProtocol {
	switch a {
	case structs.ListenerProtocolHTTP:
		return APIGatewayListenerProtocol_ListenerProtocolHTTP
	case structs.ListenerProtocolTCP:
		return APIGatewayListenerProtocol_ListenerProtocolTCP
	default:
		return APIGatewayListenerProtocol_ListenerProtocolHTTP
	}
}

func apiGatewayProtocolToStructs(a APIGatewayListenerProtocol) structs.APIGatewayListenerProtocol {
	switch a {
	case APIGatewayListenerProtocol_ListenerProtocolHTTP:
		return structs.ListenerProtocolHTTP
	case APIGatewayListenerProtocol_ListenerProtocolTCP:
		return structs.ListenerProtocolTCP
	default:
		return structs.ListenerProtocolHTTP
	}
}

func httpMatchMethodFromStructs(a structs.HTTPMatchMethod) HTTPMatchMethod {
	switch a {
	case structs.HTTPMatchMethodAll:
		return HTTPMatchMethod_HTTPMatchMethodAll
	case structs.HTTPMatchMethodConnect:
		return HTTPMatchMethod_HTTPMatchMethodConnect
	case structs.HTTPMatchMethodDelete:
		return HTTPMatchMethod_HTTPMatchMethodDelete
	case structs.HTTPMatchMethodGet:
		return HTTPMatchMethod_HTTPMatchMethodGet
	case structs.HTTPMatchMethodHead:
		return HTTPMatchMethod_HTTPMatchMethodHead
	case structs.HTTPMatchMethodOptions:
		return HTTPMatchMethod_HTTPMatchMethodOptions
	case structs.HTTPMatchMethodPatch:
		return HTTPMatchMethod_HTTPMatchMethodPatch
	case structs.HTTPMatchMethodPost:
		return HTTPMatchMethod_HTTPMatchMethodPost
	case structs.HTTPMatchMethodPut:
		return HTTPMatchMethod_HTTPMatchMethodPut
	case structs.HTTPMatchMethodTrace:
		return HTTPMatchMethod_HTTPMatchMethodTrace
	default:
		return HTTPMatchMethod_HTTPMatchMethodAll
	}
}

func httpMatchMethodToStructs(a HTTPMatchMethod) structs.HTTPMatchMethod {
	switch a {
	case HTTPMatchMethod_HTTPMatchMethodAll:
		return structs.HTTPMatchMethodAll
	case HTTPMatchMethod_HTTPMatchMethodConnect:
		return structs.HTTPMatchMethodConnect
	case HTTPMatchMethod_HTTPMatchMethodDelete:
		return structs.HTTPMatchMethodDelete
	case HTTPMatchMethod_HTTPMatchMethodGet:
		return structs.HTTPMatchMethodGet
	case HTTPMatchMethod_HTTPMatchMethodHead:
		return structs.HTTPMatchMethodHead
	case HTTPMatchMethod_HTTPMatchMethodOptions:
		return structs.HTTPMatchMethodOptions
	case HTTPMatchMethod_HTTPMatchMethodPatch:
		return structs.HTTPMatchMethodPatch
	case HTTPMatchMethod_HTTPMatchMethodPost:
		return structs.HTTPMatchMethodPost
	case HTTPMatchMethod_HTTPMatchMethodPut:
		return structs.HTTPMatchMethodPut
	case HTTPMatchMethod_HTTPMatchMethodTrace:
		return structs.HTTPMatchMethodTrace
	default:
		return structs.HTTPMatchMethodAll
	}
}

func httpHeaderMatchFromStructs(a structs.HTTPHeaderMatchType) HTTPHeaderMatchType {
	switch a {
	case structs.HTTPHeaderMatchExact:
		return HTTPHeaderMatchType_HTTPHeaderMatchExact
	case structs.HTTPHeaderMatchPrefix:
		return HTTPHeaderMatchType_HTTPHeaderMatchPrefix
	case structs.HTTPHeaderMatchPresent:
		return HTTPHeaderMatchType_HTTPHeaderMatchPresent
	case structs.HTTPHeaderMatchRegularExpression:
		return HTTPHeaderMatchType_HTTPHeaderMatchRegularExpression
	case structs.HTTPHeaderMatchSuffix:
		return HTTPHeaderMatchType_HTTPHeaderMatchSuffix
	default:
		return HTTPHeaderMatchType_HTTPHeaderMatchExact
	}
}

func httpHeaderMatchToStructs(a HTTPHeaderMatchType) structs.HTTPHeaderMatchType {
	switch a {
	case HTTPHeaderMatchType_HTTPHeaderMatchExact:
		return structs.HTTPHeaderMatchExact
	case HTTPHeaderMatchType_HTTPHeaderMatchPrefix:
		return structs.HTTPHeaderMatchPrefix
	case HTTPHeaderMatchType_HTTPHeaderMatchPresent:
		return structs.HTTPHeaderMatchPresent
	case HTTPHeaderMatchType_HTTPHeaderMatchRegularExpression:
		return structs.HTTPHeaderMatchRegularExpression
	case HTTPHeaderMatchType_HTTPHeaderMatchSuffix:
		return structs.HTTPHeaderMatchSuffix
	default:
		return structs.HTTPHeaderMatchExact
	}
}

func httpPathMatchFromStructs(a structs.HTTPPathMatchType) HTTPPathMatchType {
	switch a {
	case structs.HTTPPathMatchExact:
		return HTTPPathMatchType_HTTPPathMatchExact
	case structs.HTTPPathMatchPrefix:
		return HTTPPathMatchType_HTTPPathMatchPrefix
	case structs.HTTPPathMatchRegularExpression:
		return HTTPPathMatchType_HTTPPathMatchRegularExpression
	default:
		return HTTPPathMatchType_HTTPPathMatchExact
	}
}

func httpPathMatchToStructs(a HTTPPathMatchType) structs.HTTPPathMatchType {
	switch a {
	case HTTPPathMatchType_HTTPPathMatchExact:
		return structs.HTTPPathMatchExact
	case HTTPPathMatchType_HTTPPathMatchPrefix:
		return structs.HTTPPathMatchPrefix
	case HTTPPathMatchType_HTTPPathMatchRegularExpression:
		return structs.HTTPPathMatchRegularExpression
	default:
		return structs.HTTPPathMatchExact
	}
}

func httpQueryMatchFromStructs(a structs.HTTPQueryMatchType) HTTPQueryMatchType {
	switch a {
	case structs.HTTPQueryMatchExact:
		return HTTPQueryMatchType_HTTPQueryMatchExact
	case structs.HTTPQueryMatchPresent:
		return HTTPQueryMatchType_HTTPQueryMatchPresent
	case structs.HTTPQueryMatchRegularExpression:
		return HTTPQueryMatchType_HTTPQueryMatchRegularExpression
	default:
		return HTTPQueryMatchType_HTTPQueryMatchExact
	}
}

func httpQueryMatchToStructs(a HTTPQueryMatchType) structs.HTTPQueryMatchType {
	switch a {
	case HTTPQueryMatchType_HTTPQueryMatchExact:
		return structs.HTTPQueryMatchExact
	case HTTPQueryMatchType_HTTPQueryMatchPresent:
		return structs.HTTPQueryMatchPresent
	case HTTPQueryMatchType_HTTPQueryMatchRegularExpression:
		return structs.HTTPQueryMatchRegularExpression
	default:
		return structs.HTTPQueryMatchExact
	}
}
