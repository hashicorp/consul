package consul

import (
	"github.com/hashicorp/consul/agent/structs"
)

func (s *Server) getSystemMetadata(key string) (string, error) {
	_, entry, err := s.fsm.State().SystemMetadataGet(nil, key)
	if err != nil {
		return "", err
	}
	if entry == nil {
		return "", nil
	}

	return entry.Value, nil
}

func (s *Server) setSystemMetadataKey(key, val string) error {
	args := &structs.SystemMetadataRequest{
		Op:    structs.SystemMetadataUpsert,
		Entry: &structs.SystemMetadataEntry{Key: key, Value: val},
	}

	_, err := s.raftApply(structs.SystemMetadataRequestType, args)
	return err
}

func (s *Server) deleteSystemMetadataKey(key string) error {
	args := &structs.SystemMetadataRequest{
		Op:    structs.SystemMetadataDelete,
		Entry: &structs.SystemMetadataEntry{Key: key},
	}

	_, err := s.raftApply(structs.SystemMetadataRequestType, args)
	return err
}
