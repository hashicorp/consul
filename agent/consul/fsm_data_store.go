package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/structs"
)

// implementation of consul/gateways/datastore
type FSMDataStore struct {
	server *Server
	fsm    *fsm.FSM
}

// GetConfigEntry takes in a kind, name, and meta and returns a configentry and an error from the FSM state
func (f *FSMDataStore) GetConfigEntry(kind string, name string, meta *acl.EnterpriseMeta) (*structs.ConfigEntry, error) {
	store := f.fsm.State()

	_, entry, err := store.ConfigEntry(nil, kind, name, meta)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetConfigEntriesByKind takes in a kind and returns all instances of that kind of config entry from the FSM state
func (f *FSMDataStore) GetConfigEntriesByKind(kind string) ([]structs.ConfigEntry, error) {
	store := f.fsm.State()

	_, entries, err := store.ConfigEntriesByKind(nil, kind, acl.WildcardEnterpriseMeta())
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// UpdateStatus updates the status of a config entry
func (f *FSMDataStore) UpdateStatus(entry structs.ConfigEntry) error {
	return nil
}

// Update takes a config entry and upserts it in the FSM state
func (f *FSMDataStore) Update(entry structs.ConfigEntry) error {
	_, err := f.server.leaderRaftApply("ConfigEntry.Apply", structs.ConfigEntryRequestType, &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryUpsertCAS,
		Entry: entry,
	})
	return err
}

// Delete takes a config entry and deletes it from the FSM state
func (f *FSMDataStore) Delete(entry structs.ConfigEntry) error {
	_, err := f.server.leaderRaftApply("ConfigEntry.Delete", structs.ConfigEntryRequestType, &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryDelete,
		Entry: entry,
	})
	return err
}
