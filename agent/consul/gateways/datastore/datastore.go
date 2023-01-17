package datastore

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

type DataStore interface {
	GetConfigEntry(kind string, name string, meta *acl.EnterpriseMeta) (structs.ConfigEntry, error)
	UpdateStatus(entry structs.ConfigEntry) error
	Update(entry structs.ConfigEntry) error
	Delete(entry structs.ConfigEntry) error
}
