// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"context"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/consul/controller"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
)

//go:generate mockery --name ceReadEntryFunc --inpackage --filename mock_ceReadEntry.go
type ceReadEntryFunc func(k string, n string) (uint64, structs.ConfigEntry, error)

//go:generate mockery --name ceUpdater --inpackage --filename mock_ceUpdater.go
type ceUpdater interface {
	UpdateGlobalRateLimitConfig(cfg *structs.GlobalRateLimitConfigEntry)
}

type rateLimiterReconciler struct {
	readEntry  ceReadEntryFunc
	logger     hclog.Logger
	controller controller.Controller
	updater    ceUpdater
}

func (r rateLimiterReconciler) Reconcile(ctx context.Context, req controller.Request) error {
	switch req.Kind {
	case structs.RateLimit:
		return reconcileEntry(r.readEntry, requestLogger(r.logger, req), ctx, req, r.updater)
	default:
		return nil
	}
}

func reconcileEntry(readEntry ceReadEntryFunc, logger hclog.Logger, _ context.Context, req controller.Request, updater ceUpdater) error {
	_, entry, err := readEntry(req.Kind, req.Name)
	if err != nil {
		logger.Warn("error fetching config entry for reconciliation request", "error", err)
		return err
	}

	// Entry is deleted — reset to empty config
	if entry == nil {
		updater.UpdateGlobalRateLimitConfig(nil)
		return nil
	}

	// Update with the actual config entry when it exists
	cfg, ok := entry.(*structs.GlobalRateLimitConfigEntry)
	if !ok {
		logger.Error("failed to cast config entry to GlobalRateLimitConfigEntry",
			"entry_type", entry.GetKind())
		return nil
	}
	updater.UpdateGlobalRateLimitConfig(cfg)
	return nil
}

// NewRateLimiterController initializes a controller that reconciles rate limiter config
func NewRateLimiterController(readEntry ceReadEntryFunc, publisher state.EventPublisher, logger hclog.Logger, updater ceUpdater) controller.Controller {
	reconciler := &rateLimiterReconciler{
		readEntry: readEntry,
		logger:    logger,
		updater:   updater,
	}
	reconciler.controller = controller.New(publisher, reconciler).
		WithLogger(logger.With("controller", "rateLimiterController"))
	return reconciler.controller.Subscribe(
		&stream.SubscribeRequest{
			Topic:   state.EventTopicGlobalRateLimit,
			Subject: stream.SubjectWildcard,
		},
	)
}

// requestLogger returns a logger with request-specific fields.
func requestLogger(logger hclog.Logger, request controller.Request) hclog.Logger {
	return logger.With("kind", request.Kind, "name", request.Name)
}
