// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package proxycfg

type TestDataSourcesEnterprise struct{}

func (*TestDataSources) buildEnterpriseSources() {}

func (*TestDataSources) fillEnterpriseDataSources(*DataSources) {}

func testConfigSnapshotFixtureEnterprise(*stateConfig) {}
