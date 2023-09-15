// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package proxycfg

type TestDataSourcesEnterprise struct{}

func (*TestDataSources) buildEnterpriseSources() {}

func (*TestDataSources) fillEnterpriseDataSources(*DataSources) {}

func testConfigSnapshotFixtureEnterprise(*stateConfig) {}
