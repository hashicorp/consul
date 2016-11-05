package file

import (
	"strings"
	"testing"
)

func BenchmarkParseInsert(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Parse(strings.NewReader(dbMiekENTNL), testzone, "stdin")
	}
}
