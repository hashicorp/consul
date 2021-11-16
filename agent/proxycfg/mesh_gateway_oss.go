//go:build !consulent
// +build !consulent

package proxycfg

import (
	"context"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cache"
)

func (s *handlerMeshGateway) initializeEntWatches(_ context.Context) error {
	return nil
}

func (s *handlerMeshGateway) handleEntUpdate(_ hclog.Logger, _ context.Context, _ cache.UpdateEvent, _ *ConfigSnapshot) error {
	return nil
}
