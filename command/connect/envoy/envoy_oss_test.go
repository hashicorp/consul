//go:build !consulent
// +build !consulent

package envoy

// enterpriseGenerateConfigTestCases returns enterprise-only configurations to
// test in TestGenerateConfig.
func enterpriseGenerateConfigTestCases() []generateConfigTestCase {
	return nil
}
