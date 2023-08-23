package version

import (
	"testing"
)

func BenchmarkGetHumanVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetHumanVersion()
	}
}
