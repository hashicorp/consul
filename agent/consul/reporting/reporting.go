package reporting

import (
	"sync"

	"github.com/hashicorp/go-hclog"
)

type ReportingManager struct {
	logger hclog.Logger
	server ServerDelegate
	EntDeps
	sync.RWMutex
}

const (
	SystemMetadataReportingProcessID = "reporting-process-id"
)

//go:generate mockery --name ServerDelegate --inpackage
type ServerDelegate interface {
	GetSystemMetadata(key string) (string, error)
	SetSystemMetadataKey(key, val string) error
	IsLeader() bool
}

func NewReportingManager(logger hclog.Logger, deps EntDeps, server ServerDelegate) *ReportingManager {
	rm := &ReportingManager{
		logger: logger.Named("reporting"),
		server: server,
	}
	err := rm.initEnterpriseReporting(deps)
	if err != nil {
		rm.logger.Error("Error initializing reporting manager", "error", err)
		return nil
	}
	return rm
}
