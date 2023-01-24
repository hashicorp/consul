package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/structs"
)

// implementation of consul/gateways/datastore
type FSMDataStore struct {
	s   *Server
	fsm *fsm.FSM
}

func (f FSMDataStore) GetConfigEntry(kind string, name string, meta *acl.EnterpriseMeta) (*structs.ConfigEntry, error) {
	store := f.fsm.State()

	_, entry, err := store.ConfigEntry(nil, kind, name, meta)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (f FSMDataStore) GetConfigEntriesByKind(kind string) ([]structs.ConfigEntry, error) {
	store := f.fsm.State()

	_, entries, err := store.ConfigEntriesByKind(nil, kind, acl.WildcardEnterpriseMeta())
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (f FSMDataStore) UpdateStatus(entry structs.ConfigEntry) error {
	return nil
}

func (f FSMDataStore) Update(entry structs.ConfigEntry) error {
	_, err := f.s.leaderRaftApply("ConfigEntry.Apply", structs.ConfigEntryRequestType, &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryUpsertCAS,
		Entry: entry,
	})
	return err
}

func (f FSMDataStore) Delete(entry structs.ConfigEntry) error {
	_, err := f.s.leaderRaftApply("ConfigEntry.Delete", structs.ConfigEntryRequestType, &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryDelete,
		Entry: entry,
	})
	return err
}
