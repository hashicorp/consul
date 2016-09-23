package kubernetes

import "testing"

// Test data for TestSymbolContainsWildcard cases.
var testdataSymbolContainsWildcard = []struct {
	Symbol         string
	ExpectedResult bool
}{
	{"mynamespace", false},
	{"*", true},
	{"any", true},
	{"my*space", true},
	{"*space", true},
	{"myname*", true},
}

func TestSymbolContainsWildcard(t *testing.T) {
	for _, example := range testdataSymbolContainsWildcard {
		actualResult := symbolContainsWildcard(example.Symbol)
		if actualResult != example.ExpectedResult {
			t.Errorf("Expected SymbolContainsWildcard result '%v' for example string='%v'. Instead got result '%v'.", example.ExpectedResult, example.Symbol, actualResult)
		}
	}
}
