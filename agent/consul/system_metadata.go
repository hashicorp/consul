package consul

import (
	"github.com/hashicorp/consul/agent/structs"
)

func (s *Server) GetSystemMetadata(key string) (string, error) {
	_, entry, err := s.fsm.State().SystemMetadataGet(nil, key)
	if err != nil {
		return "", err
	}
	if entry == nil {
		return "", nil
	}

	return entry.Value, nil
}

func (s *Server) SetSystemMetadataKey(key, val string) error {
	args := &structs.SystemMetadataRequest{
		Op:    structs.SystemMetadataUpsert,
		Entry: &structs.SystemMetadataEntry{Key: key, Value: val},
	}

	// TODO(rpc-metrics-improv): Double check request name here
	_, err := s.leaderRaftApply("SystemMetadata.Upsert", structs.SystemMetadataRequestType, args)

	return err
}

func (s *Server) deleteSystemMetadataKey(key string) error {
	args := &structs.SystemMetadataRequest{
		Op:    structs.SystemMetadataDelete,
		Entry: &structs.SystemMetadataEntry{Key: key},
	}

	// TODO(rpc-metrics-improv): Double check request name here
	_, err := s.leaderRaftApply("SystemMetadata.Delete", structs.SystemMetadataRequestType, args)

	return err
}
