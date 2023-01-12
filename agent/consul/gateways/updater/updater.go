package updater

import "github.com/hashicorp/consul/agent/structs"

type Updater interface {
	UpdateStatus(entry structs.ConfigEntry) error
	Update(entry structs.ConfigEntry) error
	Delete(entry structs.ConfigEntry) error
}
