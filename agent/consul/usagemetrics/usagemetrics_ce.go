// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package usagemetrics

func newTenancyUsageReporter(u *UsageMetricsReporter) usageReporter {
	return newBaseUsageReporter(u)
}
