package configentry

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// KindName is a value type useful for maps. You can use:
//
//	map[KindName]Payload
//
// instead of:
//
//	map[string]map[string]Payload
type KindName struct {
	Kind string
	Name string
	acl.EnterpriseMeta
}

// NewKindName returns a new KindName. The EnterpriseMeta values will be
// normalized based on the kind.
//
// Any caller which modifies the EnterpriseMeta field must call Normalize
// before persisting or using the value as a map key.
func NewKindName(kind, name string, entMeta *acl.EnterpriseMeta) KindName {
	ret := KindName{
		Kind: kind,
		Name: name,
	}
	if entMeta == nil {
		entMeta = structs.DefaultEnterpriseMetaInDefaultPartition()
	}

	ret.EnterpriseMeta = *entMeta
	ret.Normalize()
	return ret
}

func NewKindNameForEntry(entry structs.ConfigEntry) KindName {
	return NewKindName(entry.GetKind(), entry.GetName(), entry.GetEnterpriseMeta())
}
