package xds

import (
	"errors"
<<<<<<< HEAD
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
=======

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/consul/agent/proxycfg"
>>>>>>> 4989268415 (Begin stubbing for SDS)
)

// secretsFromSnapshot returns the xDS API representation of the "secrets"
// in the snapshot
<<<<<<< HEAD
func (s *ResourceGenerator) secretsFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy,
		structs.ServiceKindTerminatingGateway,
		structs.ServiceKindMeshGateway,
		structs.ServiceKindIngressGateway:
		return nil, nil
	// Only API gateways utilize secrets
	case structs.ServiceKindAPIGateway:
		return s.secretsFromSnapshotAPIGateway(cfgSnap)
	default:
		return nil, fmt.Errorf("Invalid service kind: %v", cfgSnap.Kind)
	}
}

func (s *ResourceGenerator) secretsFromSnapshotAPIGateway(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	var res []proto.Message
	// TODO
	return res, nil
}

// TODO Implement
func (s *ResourceGenerator) secretsFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	return nil, errors.New("implement me")
}
