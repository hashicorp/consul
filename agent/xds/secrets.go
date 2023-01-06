package xds

import (
	"errors"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/consul/agent/proxycfg"
)

// secretsFromSnapshot returns the xDS API representation of the "secrets"
// in the snapshot
// TODO Implement
func (s *ResourceGenerator) secretsFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot) ([]proto.Message, error) {
	return nil, errors.New("implement me")
}
