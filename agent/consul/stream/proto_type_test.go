package stream

import "testing"

var result interface{}

func BenchmarkEmptyUntypedMapMarshal(b *testing.B) {
	var m UntypedMap

	for i := 0; i < b.N; i++ {
		bs, _ := m.Marshal()
		result = bs // Ensure compiler can't elide the loop body by making the result visible outside loop
	}
}

func BenchmarkFullUntypedMapMarshal(b *testing.B) {
	m := UntypedMap{
		"one": map[string]interface{}{
			"two":  3,
			"four": "Five",
		},
		"a really long key name": []string{"several longer strings", "more longer strings", "long long long"},
	}

	for i := 0; i < b.N; i++ {
		bs, _ := m.Marshal()
		result = bs // Ensure compiler can't elide the loop body by making the result visible outside loop
	}
}
