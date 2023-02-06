package gateways

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

//go:generate mockery --name DataStore --inpackage
type DataStore interface {
	GetConfigEntry(kind string, name string, meta *acl.EnterpriseMeta) (structs.ConfigEntry, error)
	GetConfigEntriesByKind(kind string) ([]structs.ConfigEntry, error)
	Update(entry structs.ConfigEntry) error
	UpdateStatus(entry structs.ControlledConfigEntry, err error) error
	Delete(entry structs.ConfigEntry) error
}
