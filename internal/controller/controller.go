package controller

import (
	"context"

	"github.com/hashicorp/go-hclog"
)

// controllerRunner contains the actual implementation of running a controller
// including creating watches, calling the reconciler, handling retries, etc.
type controllerRunner struct {
	ctrl   Controller
	logger hclog.Logger
}

func (c *controllerRunner) run(ctx context.Context) error {
	c.logger.Debug("controller running")
	defer c.logger.Debug("controller stopping")

	<-ctx.Done()
	return ctx.Err()
}
