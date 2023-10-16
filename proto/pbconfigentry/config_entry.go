package pbconfigentry

import (
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/types"
)

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
	case Kind_KindServiceDefaults:
		var target structs.ServiceConfigEntry
		target.Name = s.Name

		ServiceDefaultsToStructs(s.GetServiceDefaults(), &target)
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

func timeFromStructs(t *time.Time) *timestamp.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func timeToStructs(ts *timestamp.Timestamp) *time.Time {
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

func intentionSourceTypeFromStructs(structs.IntentionSourceType) IntentionSourceType {
	return IntentionSourceType_Consul
}

func intentionSourceTypeToStructs(IntentionSourceType) structs.IntentionSourceType {
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
