package consul

import "github.com/hashicorp/consul/agent/structs"

type FSMDataStore struct {
	s *Server
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
