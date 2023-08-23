package pbpeerstream

import (
	"fmt"

	"github.com/hashicorp/consul/agent/structs"
	pbservice "github.com/hashicorp/consul/proto/pbservice"
)

// CheckServiceNodesToStruct converts the contained CheckServiceNodes to their structs equivalent.
func (s *ExportedService) CheckServiceNodesToStruct() ([]structs.CheckServiceNode, error) {
	if s == nil {
		return nil, nil
	}

	resp := make([]structs.CheckServiceNode, 0, len(s.Nodes))
	for _, pb := range s.Nodes {
		instance, err := pbservice.CheckServiceNodeToStructs(pb)
		if err != nil {
			return resp, fmt.Errorf("failed to convert instance: %w", err)
		}
		resp = append(resp, *instance)
	}
	return resp, nil
}
