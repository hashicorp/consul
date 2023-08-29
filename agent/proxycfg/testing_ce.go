//go:build !consulent
// +build !consulent

package proxycfg

type TestDataSourcesEnterprise struct{}

func (*TestDataSources) buildEnterpriseSources() {}

func (*TestDataSources) fillEnterpriseDataSources(*DataSources) {}

func testConfigSnapshotFixtureEnterprise(*stateConfig) {}
