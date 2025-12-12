// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package proxycfg

type TestDataSourcesEnterprise struct{}

func (*TestDataSources) buildEnterpriseSources() {}

func (*TestDataSources) fillEnterpriseDataSources(*DataSources) {}

func testConfigSnapshotFixtureEnterprise(*stateConfig) {}
