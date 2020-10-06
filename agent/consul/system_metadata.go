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

	resp, err := s.raftApply(structs.SystemMetadataRequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}

func (s *Server) deleteSystemMetadataKey(key string) error {
	args := &structs.SystemMetadataRequest{
		Op:    structs.SystemMetadataDelete,
		Entry: &structs.SystemMetadataEntry{Key: key},
	}

	resp, err := s.raftApply(structs.SystemMetadataRequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}
