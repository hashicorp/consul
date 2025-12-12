// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package version

import (
	"testing"
)

func BenchmarkGetHumanVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetHumanVersion()
	}
}
