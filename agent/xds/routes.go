package xds

import (
	"errors"

	"github.com/gogo/protobuf/proto"

	"github.com/hashicorp/consul/agent/proxycfg"
)

// routesFromSnapshot returns the xDS API representation of the "routes"
// in the snapshot.
func routesFromSnapshot(cfgSnap *proxycfg.ConfigSnapshot, token string) ([]proto.Message, error) {
	if cfgSnap == nil {
		return nil, errors.New("nil config given")
	}
	// We don't support routes yet but probably will later
	return nil, nil
}
