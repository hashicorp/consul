// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package reporting

import (
	"context"
)

type EntDeps struct{}

func (rm *ReportingManager) initEnterpriseReporting(entDeps EntDeps) error {
	// no op
	return nil
}

func (rm *ReportingManager) StartReportingAgent() error {
	// no op
	return nil
}

func (rm *ReportingManager) StopReportingAgent() error {
	// no op
	return nil
}

func (rm *ReportingManager) RunMetricsWriter(ctx context.Context) {
	// no op
}

func (rm *ReportingManager) RunManualSnapshotWriter(ctx context.Context) {
	// no op
}
