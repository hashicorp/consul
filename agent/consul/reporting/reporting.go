// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package reporting

import (
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

type ReportingManager struct {
	logger         hclog.Logger
	server         ServerDelegate
	stateProvider  StateDelegate
	tickerInterval time.Duration
	EntDeps
	sync.RWMutex
}

const (
	SystemMetadataReportingProcessID = "reporting-process-id"
	ReportingInterval                = 1 * time.Hour
)

//go:generate mockery --name ServerDelegate --inpackage
type ServerDelegate interface {
	GetSystemMetadata(key string) (string, error)
	SetSystemMetadataKey(key, val string) error
	IsLeader() bool
}

type StateDelegate interface {
	NodeUsage() (uint64, state.NodeUsage, error)
	ServiceUsage(ws memdb.WatchSet) (uint64, structs.ServiceUsage, error)
}

func NewReportingManager(logger hclog.Logger, deps EntDeps, server ServerDelegate, stateProvider StateDelegate) *ReportingManager {
	rm := &ReportingManager{
		logger:         logger.Named("reporting"),
		server:         server,
		stateProvider:  stateProvider,
		tickerInterval: ReportingInterval,
	}
	err := rm.initEnterpriseReporting(deps)
	if err != nil {
		rm.logger.Error("Error initializing reporting manager", "error", err)
		return nil
	}
	return rm
}
