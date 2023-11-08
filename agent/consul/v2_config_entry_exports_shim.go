package consul

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

type V1ServiceExportsShim struct {
	s *Server
}

func (s *V1ServiceExportsShim) GetExportedServicesConfigEntry(_ context.Context, name string, entMeta *acl.EnterpriseMeta) (*structs.ExportedServicesConfigEntry, error) {
	_, entry, err := s.s.fsm.State().ConfigEntry(nil, structs.ExportedServices, name, entMeta)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	exp, ok := entry.(*structs.ExportedServicesConfigEntry)
	if !ok {
		return nil, fmt.Errorf("exported services config entry is the wrong type: expected ExportedServicesConfigEntry, actual: %T", entry)
	}

	return exp, nil
}

func (s *V1ServiceExportsShim) WriteExportedServicesConfigEntry(_ context.Context, cfg *structs.ExportedServicesConfigEntry) error {
	if err := cfg.Normalize(); err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return err
	}

	req := &structs.ConfigEntryRequest{
		Op:    structs.ConfigEntryUpsert,
		Entry: cfg,
	}

	_, err := s.s.raftApply(structs.ConfigEntryRequestType, req)
	return err
}

func (s *V1ServiceExportsShim) DeleteExportedServicesConfigEntry(_ context.Context, name string, entMeta *acl.EnterpriseMeta) error {
	req := &structs.ConfigEntryRequest{
		Op: structs.ConfigEntryDelete,
		Entry: &structs.ExportedServicesConfigEntry{
			Name:           name,
			EnterpriseMeta: *entMeta,
		},
	}

	if err := req.Entry.Normalize(); err != nil {
		return err
	}

	_, err := s.s.raftApply(structs.ConfigEntryRequestType, req)
	return err
}
